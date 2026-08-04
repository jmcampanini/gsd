package project

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
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
	if err := domain.ValidateTitle(fields.Title); err != nil {
		return Project{}, err
	}
	if err := domain.ValidateNote(fields.Note); err != nil {
		return Project{}, err
	}
	if err := validateAreaID(fields.AreaID); err != nil {
		return Project{}, err
	}

	var err error
	fields.Tags, err = domain.NormalizeTagNames(fields.Tags)
	if err != nil {
		return Project{}, err
	}
	timestamp := domain.FormatTimestamp(s.now())
	if len(fields.Tags) == 0 {
		return s.store.Add(ctx, fields, timestamp)
	}

	var created Project
	err = s.store.WithinTransaction(ctx, func(store Transaction) error {
		created, err = store.Add(ctx, fields, timestamp)
		if err != nil {
			return err
		}

		resolved, err := store.ResolveTags(ctx, fields.Tags)
		if err != nil {
			return err
		}
		if err := store.AttachTags(ctx, created.ID, resolved); err != nil {
			return err
		}

		created, err = store.Find(ctx, created.ID)
		return err
	})
	if err != nil {
		return Project{}, err
	}

	return created, nil
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Project, error) {
	if !validListStatus(options.Status) {
		return nil, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid project list status %q", options.Status),
			nil,
		)
	}
	if err := validateAreaID(options.AreaID); err != nil {
		return nil, err
	}

	return domain.NormalizeSliceResult(s.store.List(ctx, options))
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
	if fields.Area.Set != nil && fields.Area.Clear {
		return Project{}, apperr.New(
			apperr.InvalidArgument,
			"area cannot be set and cleared",
			nil,
		)
	}
	if err := validateAreaID(fields.Area.Set); err != nil {
		return Project{}, err
	}
	if fields.Title == nil && fields.Note == nil && fields.Area.Set == nil && !fields.Area.Clear {
		return Project{}, apperr.New(
			apperr.InvalidArgument,
			"project edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := domain.ValidateTitle(*fields.Title); err != nil {
			return Project{}, err
		}
	}
	if fields.Note != nil {
		if err := domain.ValidateNote(*fields.Note); err != nil {
			return Project{}, err
		}
	}

	return s.store.Edit(ctx, id, fields, domain.FormatTimestamp(s.now()))
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

	timestamp := domain.FormatTimestamp(s.now())
	resolution := Resolution{CancelledTasks: []task.Task{}}
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		project, err := store.Resolve(ctx, id, exit, timestamp)
		if err != nil {
			return err
		}

		cancelledTasks, err := domain.NormalizeSliceResult(store.CancelOpenTasks(ctx, id, timestamp))
		if err != nil {
			return err
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

	return s.store.Reopen(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Tag(ctx context.Context, id int64, names []string) (Tagging, error) {
	return s.changeTags(ctx, id, names, true)
}

func (s *Service) Untag(ctx context.Context, id int64, names []string) (Tagging, error) {
	return s.changeTags(ctx, id, names, false)
}

func (s *Service) changeTags(
	ctx context.Context,
	id int64,
	names []string,
	attach bool,
) (Tagging, error) {
	if err := validateID(id); err != nil {
		return Tagging{}, err
	}
	if len(names) == 0 {
		return Tagging{}, apperr.New(
			apperr.InvalidArgument,
			"project tagging requires at least one tag",
			nil,
		)
	}

	normalizedNames, normalizeErr := domain.NormalizeTagNames(names)
	if normalizeErr != nil {
		return Tagging{}, normalizeErr
	}

	var result Tagging
	transactionErr := s.store.WithinTransaction(ctx, func(store Transaction) error {
		if _, err := store.Find(ctx, id); err != nil {
			return err
		}

		resolvedTags, err := store.ResolveTags(ctx, normalizedNames)
		if err != nil {
			return err
		}
		if attach {
			if err := store.AttachTags(ctx, id, resolvedTags); err != nil {
				return err
			}
		} else if err := store.DetachTags(ctx, id, resolvedTags); err != nil {
			return err
		}

		refreshed, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		result = Tagging{Project: refreshed, TagTitles: tag.Titles(resolvedTags)}
		return nil
	})
	if transactionErr != nil {
		return Tagging{}, transactionErr
	}

	return result, nil
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

		return Deletion{
			Project:      project,
			DeletedTasks: []task.Task{},
		}, nil
	}

	deletion := Deletion{DeletedTasks: []task.Task{}}
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		deletedTasks, err := domain.NormalizeSliceResult(store.DeleteTasks(ctx, id))
		if err != nil {
			return err
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
	return domain.ParseID("project", value)
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
	return domain.ValidateID("project", id)
}

func validateAreaID(id *int64) error {
	return domain.ValidateOptionalID("area", id)
}
