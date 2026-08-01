package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Project, error) {
	if err := validateTitle(fields.Title); err != nil {
		return Project{}, err
	}
	if err := validateNote(fields.Note); err != nil {
		return Project{}, err
	}

	return s.store.Add(ctx, fields, formatTimestamp(s.now()))
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Project, error) {
	if !validListStatus(options.Status) {
		return nil, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid project list status %q", options.Status),
			nil,
		)
	}

	projects, err := s.store.List(ctx, options)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		return []Project{}, nil
	}

	return projects, nil
}

func (s *Service) Show(ctx context.Context, id int64) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}

	return s.store.Find(ctx, id)
}

func (s *Service) Edit(ctx context.Context, id int64, fields EditFields) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}
	if fields.Title == nil && fields.Note == nil {
		return Project{}, apperr.New(
			apperr.InvalidArgument,
			"project edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := validateTitle(*fields.Title); err != nil {
			return Project{}, err
		}
	}
	if fields.Note != nil {
		if err := validateNote(*fields.Note); err != nil {
			return Project{}, err
		}
	}

	return s.store.Edit(ctx, id, fields, formatTimestamp(s.now()))
}

func (s *Service) Resolve(ctx context.Context, id int64, exit Exit) (Resolution, error) {
	if err := validateID(id); err != nil {
		return Resolution{}, err
	}
	if !validExit(exit) {
		return Resolution{}, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid project exit %q", exit),
			nil,
		)
	}

	timestamp := formatTimestamp(s.now())
	resolution := Resolution{CancelledTasks: []task.Task{}}
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		project, err := store.Resolve(ctx, id, exit, timestamp)
		if err != nil {
			return err
		}

		cancelledTasks, err := store.CancelOpenTasks(ctx, id, timestamp)
		if err != nil {
			return err
		}
		if cancelledTasks == nil {
			cancelledTasks = []task.Task{}
		}

		resolution.Project = project
		resolution.CancelledTasks = cancelledTasks
		return nil
	})
	if err != nil {
		return Resolution{}, err
	}

	return resolution, nil
}

func (s *Service) Reopen(ctx context.Context, id int64) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}

	return s.store.Reopen(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Delete(ctx context.Context, id int64, recursive bool) (Deletion, error) {
	if err := validateID(id); err != nil {
		return Deletion{}, err
	}

	if !recursive {
		project, err := s.store.Delete(ctx, id)
		if err != nil {
			return Deletion{}, err
		}

		return Deletion{Project: project, DeletedTasks: []task.Task{}}, nil
	}

	deletion := Deletion{DeletedTasks: []task.Task{}}
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		deletedTasks, err := store.DeleteTasks(ctx, id)
		if err != nil {
			return err
		}
		if deletedTasks == nil {
			deletedTasks = []task.Task{}
		}

		project, err := store.Delete(ctx, id)
		if err != nil {
			return err
		}

		deletion.Project = project
		deletion.DeletedTasks = deletedTasks
		return nil
	})
	if err != nil {
		return Deletion{}, err
	}

	return deletion, nil
}

func ParseID(value string) (int64, error) {
	if value == "" {
		return 0, apperr.New(apperr.InvalidArgument, "project ID must be a positive decimal", nil)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("invalid project ID %q", value),
				nil,
			)
		}
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid project ID %q", value),
			err,
		)
	}

	return id, nil
}

func ParseListStatus(value string) (ListStatus, error) {
	status := ListStatus(value)
	if !validListStatus(status) {
		return "", apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid project list status %q", value),
			nil,
		)
	}

	return status, nil
}

func validListStatus(status ListStatus) bool {
	switch status {
	case ListStatusOpen, ListStatusDone, ListStatusCancelled, ListStatusAll:
		return true
	default:
		return false
	}
}

func validExit(exit Exit) bool {
	switch exit {
	case ExitDone, ExitCancelled:
		return true
	default:
		return false
	}
}

func validateID(id int64) error {
	if id <= 0 {
		return apperr.New(apperr.InvalidArgument, "project ID must be positive", nil)
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

func validateNote(note string) error {
	if !utf8.ValidString(note) {
		return apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}

	return nil
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}
