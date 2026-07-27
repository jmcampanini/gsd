package task

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository, now: time.Now}
}

func (s *Service) Add(ctx context.Context, title, note string) (Task, error) {
	if err := validateTitle(title); err != nil {
		return Task{}, err
	}
	if !utf8.ValidString(note) {
		return Task{}, NewError(ErrorInvalidArgument, "note must be valid UTF-8", nil)
	}

	return s.repository.Add(ctx, title, note, formatTimestamp(s.now()))
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
	if id <= 0 {
		return Task{}, NewError(ErrorInvalidArgument, "task ID must be positive", nil)
	}

	return s.repository.Find(ctx, id)
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
	return value.UTC().Truncate(time.Millisecond).Format("2006-01-02T15:04:05.000Z")
}
