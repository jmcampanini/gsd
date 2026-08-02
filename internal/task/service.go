package task

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/dates"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Task, error) {
	if err := validateTitle(fields.Title); err != nil {
		return Task{}, err
	}
	if !utf8.ValidString(fields.Note) {
		return Task{}, apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}
	if fields.ProjectID != nil && *fields.ProjectID <= 0 {
		return Task{}, apperr.New(apperr.InvalidArgument, "project ID must be positive", nil)
	}
	if fields.AreaID != nil && *fields.AreaID <= 0 {
		return Task{}, apperr.New(apperr.InvalidArgument, "area ID must be positive", nil)
	}
	if fields.ProjectID != nil && fields.AreaID != nil {
		return Task{}, apperr.New(apperr.InvalidArgument, "task cannot belong to both a project and an area", nil)
	}

	reference := s.now()
	var err error
	fields.DueOn, err = canonicalizeDate(fields.DueOn, reference)
	if err != nil {
		return Task{}, err
	}
	fields.DeferUntil, err = canonicalizeDate(fields.DeferUntil, reference)
	if err != nil {
		return Task{}, err
	}

	return s.store.Add(ctx, fields, formatTimestamp(reference))
}

func (s *Service) Inbox(ctx context.Context) ([]ViewTask, error) {
	return normalizeSlice(s.store.Inbox(ctx))
}

func (s *Service) Available(ctx context.Context) ([]ViewTask, error) {
	return normalizeSlice(s.store.Available(ctx))
}

func (s *Service) Show(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Task, error) {
	if !validListStatus(options.Status) {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid list status %q", options.Status), nil)
	}
	if !validDateSelector(options.Date) {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid date selector %q", options.Date), nil)
	}
	if options.ProjectID != nil && *options.ProjectID <= 0 {
		return nil, apperr.New(apperr.InvalidArgument, "project ID must be positive", nil)
	}
	if options.AreaID != nil && *options.AreaID <= 0 {
		return nil, apperr.New(apperr.InvalidArgument, "area ID must be positive", nil)
	}
	if options.ProjectID != nil && options.AreaID != nil {
		return nil, apperr.New(apperr.InvalidArgument, "cannot filter tasks by both project and area", nil)
	}

	return normalizeSlice(s.store.List(ctx, options))
}

func (s *Service) Edit(ctx context.Context, id int64, fields EditFields) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}
	if fields.DueOn.Set != nil && fields.DueOn.Clear {
		return Task{}, apperr.New(apperr.InvalidArgument, "due date cannot be set and cleared", nil)
	}
	if fields.DeferUntil.Set != nil && fields.DeferUntil.Clear {
		return Task{}, apperr.New(apperr.InvalidArgument, "defer date cannot be set and cleared", nil)
	}
	if fields.Project.Set != nil && fields.Project.Clear {
		return Task{}, apperr.New(apperr.InvalidArgument, "project cannot be set and cleared", nil)
	}
	if fields.Project.Set != nil && *fields.Project.Set <= 0 {
		return Task{}, apperr.New(apperr.InvalidArgument, "project ID must be positive", nil)
	}
	if fields.Area.Set != nil && fields.Area.Clear {
		return Task{}, apperr.New(apperr.InvalidArgument, "area cannot be set and cleared", nil)
	}
	if fields.Area.Set != nil && *fields.Area.Set <= 0 {
		return Task{}, apperr.New(apperr.InvalidArgument, "area ID must be positive", nil)
	}
	if fields.Project.Set != nil && fields.Area.Set != nil {
		return Task{}, apperr.New(apperr.InvalidArgument, "task cannot be moved to both a project and an area", nil)
	}
	if fields.Title == nil && fields.Note == nil &&
		fields.DueOn.Set == nil && !fields.DueOn.Clear &&
		fields.DeferUntil.Set == nil && !fields.DeferUntil.Clear &&
		fields.Project.Set == nil && !fields.Project.Clear &&
		fields.Area.Set == nil && !fields.Area.Clear {
		return Task{}, apperr.New(
			apperr.InvalidArgument,
			"task edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := validateTitle(*fields.Title); err != nil {
			return Task{}, err
		}
	}
	if fields.Note != nil && !utf8.ValidString(*fields.Note) {
		return Task{}, apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}

	reference := s.now()
	var err error
	fields.DueOn.Set, err = canonicalizeDate(fields.DueOn.Set, reference)
	if err != nil {
		return Task{}, err
	}
	fields.DeferUntil.Set, err = canonicalizeDate(fields.DeferUntil.Set, reference)
	if err != nil {
		return Task{}, err
	}

	return s.store.Edit(ctx, id, fields, formatTimestamp(reference))
}

func (s *Service) Done(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Done(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Cancel(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Cancel(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Reopen(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Reopen(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Delete(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Delete(ctx, id)
}

func ParseListStatus(value string) (ListStatus, error) {
	status := ListStatus(value)
	if !validListStatus(status) {
		return "", apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid list status %q", value), nil)
	}

	return status, nil
}

func ParseID(value string) (int64, error) {
	if value == "" {
		return 0, apperr.New(apperr.InvalidArgument, "task ID must be a positive decimal", nil)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid task ID %q", value), nil)
		}
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid task ID %q", value), err)
	}

	return id, nil
}

func validListStatus(status ListStatus) bool {
	switch status {
	case ListStatusOpen, ListStatusDone, ListStatusCancelled, ListStatusAll:
		return true
	default:
		return false
	}
}

func validDateSelector(selector DateSelector) bool {
	switch selector {
	case DateSelectorNone, DateSelectorDue, DateSelectorOverdue, DateSelectorDeferred:
		return true
	default:
		return false
	}
}

func canonicalizeDate(value *string, reference time.Time) (*string, error) {
	if value == nil {
		return nil, nil
	}

	canonical, err := dates.Parse(*value, reference)
	if err != nil {
		return nil, apperr.New(apperr.InvalidArgument, fmt.Sprintf("invalid date %q", *value), err)
	}

	return &canonical, nil
}

func normalizeSlice[T any](values []T, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	if values == nil {
		return []T{}, nil
	}

	return values, nil
}

func validateID(id int64) error {
	if id <= 0 {
		return apperr.New(apperr.InvalidArgument, "task ID must be positive", nil)
	}

	return nil
}

func validateTitle(title string) error {
	if !utf8.ValidString(title) {
		return apperr.New(apperr.InvalidArgument, "title must be valid UTF-8", nil)
	}
	if strings.TrimSpace(title) == "" {
		return apperr.New(apperr.InvalidArgument, "title must not be blank", nil)
	}

	return nil
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}
