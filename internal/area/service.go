package area

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
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

func (s *Service) Add(ctx context.Context, fields AddFields) (Area, error) {
	if err := domain.ValidateTitle(fields.Title); err != nil {
		return Area{}, err
	}
	if err := domain.ValidateNote(fields.Note); err != nil {
		return Area{}, err
	}
	normalizedTags, err := domain.NormalizeTagNames(fields.Tags)
	if err != nil {
		return Area{}, err
	}
	fields.Tags = normalizedTags
	timestamp := domain.FormatTimestamp(s.now())
	if len(fields.Tags) == 0 {
		return normalizeAreaResult(s.store.Add(ctx, fields, timestamp))
	}

	var added Area
	err = s.store.WithinTransaction(ctx, func(store Store) error {
		created, err := store.Add(ctx, fields, timestamp)
		if err != nil {
			return err
		}

		resolvedTags, err := store.ResolveTags(ctx, fields.Tags)
		if err != nil {
			return err
		}
		if err := store.AttachTags(ctx, created.ID, resolvedTags); err != nil {
			return err
		}

		added, err = store.Find(ctx, created.ID)
		return err
	})
	if err != nil {
		return Area{}, err
	}

	return normalizeArea(added), nil
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

	return normalizeAreasResult(s.store.List(ctx, options))
}

func (s *Service) Show(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return normalizeAreaResult(s.store.Find(ctx, id))
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

	return normalizeAreaResult(s.store.Edit(ctx, id, fields, domain.FormatTimestamp(s.now())))
}

func (s *Service) Archive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return normalizeAreaResult(s.store.Archive(ctx, id, domain.FormatTimestamp(s.now())))
}

func (s *Service) Unarchive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return normalizeAreaResult(s.store.Unarchive(ctx, id, domain.FormatTimestamp(s.now())))
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
			"area tagging requires at least one tag",
			nil,
		)
	}

	normalizedNames, err := domain.NormalizeTagNames(names)
	if err != nil {
		return Tagging{}, err
	}

	var result Tagging
	err = s.store.WithinTransaction(ctx, func(store Store) error {
		if _, err := store.Find(ctx, id); err != nil {
			return err
		}

		resolvedTags, err := store.ResolveTags(ctx, normalizedNames)
		if err != nil {
			return err
		}
		if attach {
			err = store.AttachTags(ctx, id, resolvedTags)
		} else {
			err = store.DetachTags(ctx, id, resolvedTags)
		}
		if err != nil {
			return err
		}

		result.Area, err = store.Find(ctx, id)
		if err != nil {
			return err
		}
		result.TagTitles = tag.Titles(resolvedTags)
		return nil
	})
	if err != nil {
		return Tagging{}, err
	}

	result.Area = normalizeArea(result.Area)
	return result, nil
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
			Area:            normalizeArea(deletedArea),
			DeletedProjects: []project.Project{},
			DeletedTasks:    []task.Task{},
		}, nil
	}

	var deletedArea Area
	deletedProjects := []project.Project{}
	deletedTasks := []task.Task{}
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		projectTasks, err := normalizeTasksResult(
			store.DeleteTasks(ctx, id, TaskDeletionScopeProject),
		)
		if err != nil {
			return err
		}

		deletedProjects, err = normalizeProjectsResult(store.DeleteProjects(ctx, id))
		if err != nil {
			return err
		}

		looseTasks, err := normalizeTasksResult(
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
		Area:            normalizeArea(deletedArea),
		DeletedProjects: deletedProjects,
		DeletedTasks:    deletedTasks,
	}, nil
}

func normalizeArea(result Area) Area {
	if result.Tags == nil {
		result.Tags = []string{}
	}
	return result
}

func normalizeAreaResult(result Area, err error) (Area, error) {
	if err != nil {
		return Area{}, err
	}
	return normalizeArea(result), nil
}

func normalizeAreasResult(results []Area, err error) ([]Area, error) {
	results, err = domain.NormalizeSliceResult(results, err)
	if err != nil {
		return nil, err
	}
	for index := range results {
		results[index] = normalizeArea(results[index])
	}
	return results, nil
}

func normalizeProjectsResult(results []project.Project, err error) ([]project.Project, error) {
	results, err = domain.NormalizeSliceResult(results, err)
	if err != nil {
		return nil, err
	}
	for index := range results {
		if results[index].Tags == nil {
			results[index].Tags = []string{}
		}
	}
	return results, nil
}

func normalizeTasksResult(results []task.Task, err error) ([]task.Task, error) {
	results, err = domain.NormalizeSliceResult(results, err)
	if err != nil {
		return nil, err
	}
	for index := range results {
		if results[index].Tags == nil {
			results[index].Tags = []string{}
		}
	}
	return results, nil
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
