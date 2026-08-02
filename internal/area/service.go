package area

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Area, error) {
	if err := domain.ValidateTitle(fields.Title); err != nil {
		return Area{}, err
	}
	if err := domain.ValidateNote(fields.Note); err != nil {
		return Area{}, err
	}

	return s.store.Add(ctx, fields, domain.FormatTimestamp(s.now()))
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]Area, error) {
	if options.Slice == "" {
		options.Slice = ListSliceActive
	}
	if !validListSlice(options.Slice) {
		return nil, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid area list slice %q", options.Slice),
			nil,
		)
	}

	return domain.NormalizeSliceResult(s.store.List(ctx, options))
}

func (s *Service) Show(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return s.store.Find(ctx, id)
}

func (s *Service) Edit(ctx context.Context, id int64, fields EditFields) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}
	if fields.Title == nil && fields.Note == nil {
		return Area{}, apperr.New(
			apperr.InvalidArgument,
			"area edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := domain.ValidateTitle(*fields.Title); err != nil {
			return Area{}, err
		}
	}
	if fields.Note != nil {
		if err := domain.ValidateNote(*fields.Note); err != nil {
			return Area{}, err
		}
	}

	return s.store.Edit(ctx, id, fields, domain.FormatTimestamp(s.now()))
}

func (s *Service) Archive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return s.store.Archive(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Unarchive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return s.store.Unarchive(ctx, id, domain.FormatTimestamp(s.now()))
}

func (s *Service) Delete(ctx context.Context, id int64, recursive bool) (Deletion, error) {
	if err := validateID(id); err != nil {
		return Deletion{}, err
	}

	if !recursive {
		deletedArea, err := s.store.Delete(ctx, id)
		if err != nil {
			return Deletion{}, err
		}

		return Deletion{
			Area:            deletedArea,
			DeletedProjects: []project.Project{},
			DeletedTasks:    []task.Task{},
		}, nil
	}

	var deletedArea Area
	deletedProjects := []project.Project{}
	deletedTasks := []task.Task{}
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		projectTasks, err := domain.NormalizeSliceResult(
			store.DeleteTasks(ctx, id, TaskDeletionScopeProject),
		)
		if err != nil {
			return err
		}

		deletedProjects, err = domain.NormalizeSliceResult(store.DeleteProjects(ctx, id))
		if err != nil {
			return err
		}

		looseTasks, err := domain.NormalizeSliceResult(
			store.DeleteTasks(ctx, id, TaskDeletionScopeLoose),
		)
		if err != nil {
			return err
		}

		deletedArea, err = store.Delete(ctx, id)
		if err != nil {
			return err
		}

		deletedTasks = append(deletedTasks, looseTasks...)
		tasksByProject := make(map[int64][]task.Task, len(deletedProjects))
		for _, projectTask := range projectTasks {
			tasksByProject[*projectTask.ProjectID] = append(
				tasksByProject[*projectTask.ProjectID],
				projectTask,
			)
		}
		for _, deletedProject := range deletedProjects {
			deletedTasks = append(deletedTasks, tasksByProject[deletedProject.ID]...)
		}
		return nil
	})
	if err != nil {
		return Deletion{}, err
	}
	return Deletion{
		Area:            deletedArea,
		DeletedProjects: deletedProjects,
		DeletedTasks:    deletedTasks,
	}, nil
}

func ParseID(value string) (int64, error) {
	return domain.ParseID("area", value)
}

func validListSlice(slice ListSlice) bool {
	switch slice {
	case ListSliceActive, ListSliceArchived, ListSliceAll:
		return true
	default:
		return false
	}
}

func validateID(id int64) error {
	return domain.ValidateID("area", id)
}
