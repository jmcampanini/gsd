package task

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/dates"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Task, error) {
	if err := validateTitle(fields.Title); err != nil {
		return Task{}, err
	}
	if !utf8.ValidString(fields.Note) {
		return Task{}, NewError(ErrorInvalidArgument, "note must be valid UTF-8", nil)
	}

	reference := s.now()
	if fields.DueOn != nil {
		canonical, err := parseDate(*fields.DueOn, reference)
		if err != nil {
			return Task{}, err
		}
		fields.DueOn = &canonical
	}

	return s.repository.Add(ctx, fields, formatTimestamp(reference))
}

func (s *Service) Inbox(ctx context.Context) ([]Task, error) {
	tasks, err := s.repository.Inbox(ctx)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return []Task{}, nil
	}

	return tasks, nil
}

func (s *Service) Show(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.repository.Find(ctx, id)
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Task, error) {
	if !validListStatus(options.Status) {
		return nil, NewError(ErrorInvalidArgument, fmt.Sprintf("invalid list status %q", options.Status), nil)
	}
	if !validDateSelector(options.Date) {
		return nil, NewError(ErrorInvalidArgument, fmt.Sprintf("invalid date selector %q", options.Date), nil)
	}

	tasks, err := s.repository.List(ctx, options)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return []Task{}, nil
	}

	return tasks, nil
}

func (s *Service) Edit(ctx context.Context, id int64, fields EditFields) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}
	if fields.DueOn.Set != nil && fields.DueOn.Clear {
		return Task{}, NewError(ErrorInvalidArgument, "due date cannot be set and cleared", nil)
	}
	if fields.Title == nil && fields.Note == nil && fields.DueOn.Set == nil && !fields.DueOn.Clear {
		return Task{}, NewError(
			ErrorInvalidArgument,
			"edit requires --title, --note, --due, or --no-due",
			nil,
		)
	}
	if fields.Title != nil {
		if err := validateTitle(*fields.Title); err != nil {
			return Task{}, err
		}
	}
	if fields.Note != nil && !utf8.ValidString(*fields.Note) {
		return Task{}, NewError(ErrorInvalidArgument, "note must be valid UTF-8", nil)
	}

	reference := s.now()
	if fields.DueOn.Set != nil {
		canonical, err := parseDate(*fields.DueOn.Set, reference)
		if err != nil {
			return Task{}, err
		}
		fields.DueOn.Set = &canonical
	}

	return s.repository.Edit(ctx, id, fields, formatTimestamp(reference))
}

func (s *Service) Done(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.repository.Done(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Cancel(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.repository.Cancel(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Reopen(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.repository.Reopen(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Delete(ctx context.Context, id int64) (Task, error) {
	if err := validateID(id); err != nil {
		return Task{}, err
	}

	return s.repository.Delete(ctx, id)
}

func ParseListStatus(value string) (ListStatus, error) {
	status := ListStatus(value)
	if !validListStatus(status) {
		return "", NewError(ErrorInvalidArgument, fmt.Sprintf("invalid list status %q", value), nil)
	}

	return status, nil
}

func ParseID(value string) (int64, error) {
	if value == "" {
		return 0, NewError(ErrorInvalidArgument, "task ID must be a positive decimal", nil)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, NewError(ErrorInvalidArgument, fmt.Sprintf("invalid task ID %q", value), nil)
		}
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, NewError(ErrorInvalidArgument, fmt.Sprintf("invalid task ID %q", value), err)
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
	case DateSelectorNone, DateSelectorDue, DateSelectorOverdue:
		return true
	default:
		return false
	}
}

func parseDate(value string, reference time.Time) (string, error) {
	canonical, err := dates.Parse(value, reference)
	if err != nil {
		return "", NewError(ErrorInvalidArgument, fmt.Sprintf("invalid date %q", value), err)
	}

	return canonical, nil
}

func validateID(id int64) error {
	if id <= 0 {
		return NewError(ErrorInvalidArgument, "task ID must be positive", nil)
	}

	return nil
}

func validateTitle(title string) error {
	if !utf8.ValidString(title) {
		return NewError(ErrorInvalidArgument, "title must be valid UTF-8", nil)
	}
	if strings.TrimSpace(title) == "" {
		return NewError(ErrorInvalidArgument, "title must not be blank", nil)
	}

	return nil
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}
