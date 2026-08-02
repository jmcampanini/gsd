package area

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
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
	if err := validateTitle(fields.Title); err != nil {
		return Area{}, err
	}
	if err := validateNote(fields.Note); err != nil {
		return Area{}, err
	}

	return s.store.Add(ctx, fields, formatTimestamp(s.now()))
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

	areas, err := s.store.List(ctx, options)
	if err != nil {
		return nil, err
	}
	if areas == nil {
		return []Area{}, nil
	}

	return areas, nil
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
		if err := validateTitle(*fields.Title); err != nil {
			return Area{}, err
		}
	}
	if fields.Note != nil {
		if err := validateNote(*fields.Note); err != nil {
			return Area{}, err
		}
	}

	return s.store.Edit(ctx, id, fields, formatTimestamp(s.now()))
}

func (s *Service) Archive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return s.store.Archive(ctx, id, formatTimestamp(s.now()))
}

func (s *Service) Unarchive(ctx context.Context, id int64) (Area, error) {
	if err := validateID(id); err != nil {
		return Area{}, err
	}

	return s.store.Unarchive(ctx, id, formatTimestamp(s.now()))
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
	var deletedProjects []project.Project
	deletedTasks := []task.Task{}
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		projectTasks, err := store.DeleteTasks(ctx, id, TaskDeletionScopeProject)
		if err != nil {
			return err
		}

		deletedProjects, err = store.DeleteProjects(ctx, id)
		if err != nil {
			return err
		}

		looseTasks, err := store.DeleteTasks(ctx, id, TaskDeletionScopeLoose)
		if err != nil {
			return err
		}

		deletedArea, err = store.Delete(ctx, id)
		if err != nil {
			return err
		}

		deletedTasks = append(deletedTasks, projectTasks...)
		deletedTasks = append(deletedTasks, looseTasks...)
		return nil
	})
	if err != nil {
		return Deletion{}, err
	}
	if deletedProjects == nil {
		deletedProjects = []project.Project{}
	}
	sort.Slice(deletedTasks, func(i, j int) bool {
		left := deletedTasks[i]
		right := deletedTasks[j]
		return left.Position < right.Position || left.Position == right.Position && left.ID < right.ID
	})

	return Deletion{
		Area:            deletedArea,
		DeletedProjects: deletedProjects,
		DeletedTasks:    deletedTasks,
	}, nil
}

func ParseID(value string) (int64, error) {
	if value == "" {
		return 0, apperr.New(apperr.InvalidArgument, "area ID must be a positive decimal", nil)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("invalid area ID %q", value),
				nil,
			)
		}
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid area ID %q", value),
			err,
		)
	}

	return id, nil
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
	if id <= 0 {
		return apperr.New(apperr.InvalidArgument, "area ID must be positive", nil)
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
