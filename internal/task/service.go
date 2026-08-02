package task

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/dates"
	"github.com/jmcampanini/gsd/internal/domain"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Task, error) {
	if err := domain.ValidateTitle(fields.Title); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateNote(fields.Note); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateOptionalID("project", fields.ProjectID); err != nil {
		return Task{}, err
	}
	if err := domain.ValidateOptionalID("area", fields.AreaID); err != nil {
		return Task{}, err
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

	return s.store.Add(ctx, fields, domain.FormatTimestamp(reference))
}

func (s *Service) Inbox(ctx context.Context) ([]ViewTask, error) {
	return domain.NormalizeSliceResult(s.store.Inbox(ctx))
}

func (s *Service) Available(ctx context.Context) ([]ViewTask, error) {
	return domain.NormalizeSliceResult(s.store.Available(ctx))
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
	if err := domain.ValidateOptionalID("project", options.ProjectID); err != nil {
		return nil, err
	}
	if err := domain.ValidateOptionalID("area", options.AreaID); err != nil {
		return nil, err
	}
	if options.ProjectID != nil && options.AreaID != nil {
		return nil, apperr.New(apperr.InvalidArgument, "cannot filter tasks by both project and area", nil)
	}

	return domain.NormalizeSliceResult(s.store.List(ctx, options))
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
	if err := domain.ValidateOptionalID("project", fields.Project.Set); err != nil {
		return Task{}, err
	}
	if fields.Area.Set != nil && fields.Area.Clear {
		return Task{}, apperr.New(apperr.InvalidArgument, "area cannot be set and cleared", nil)
	}
	if err := domain.ValidateOptionalID("area", fields.Area.Set); err != nil {
		return Task{}, err
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
		if err := domain.ValidateTitle(*fields.Title); err != nil {
			return Task{}, err
		}
	}
	if fields.Note != nil {
		if err := domain.ValidateNote(*fields.Note); err != nil {
			return Task{}, err
		}
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

	return s.store.Edit(ctx, id, fields, domain.FormatTimestamp(reference))
}

func (s *Service) Done(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Done(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Cancel(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Cancel(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Reopen(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.store.Reopen(ctx, id, domain.FormatTimestamp(s.now()))
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
	return domain.ParseID("task", value)
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

func validateID(id int64) error {
	return domain.ValidateID("task", id)
}
