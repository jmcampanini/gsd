package project

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
	if fields.Board != nil {
		if err := domain.ValidateTitle(*fields.Board); err != nil {
			return Project{}, err
		}
	}

	var err error
	fields.Tags, err = domain.NormalizeTagNames(fields.Tags)
	if err != nil {
		return Project{}, err
	}
	timestamp := domain.FormatTimestamp(s.now())
	create := CreateFields{
		AreaID: fields.AreaID,
		Title:  fields.Title,
		Note:   fields.Note,
	}
	if len(fields.Tags) == 0 && fields.AreaID == nil && fields.Board == nil {
		return s.store.Add(ctx, create, timestamp)
	}

	var created Project
	err = s.store.WithinTransaction(ctx, func(store Transaction) error {
		if fields.AreaID != nil {
			foundArea, err := store.FindArea(ctx, *fields.AreaID)
			if err != nil {
				return err
			}
			if err := containmentConflict("add project", Project{}, foundArea); err != nil {
				return err
			}
		}
		if fields.Board != nil {
			foundBoard, err := store.FindBoard(ctx, *fields.Board)
			if err != nil {
				return err
			}
			firstStage, err := store.FindFirstStage(ctx, foundBoard.ID)
			if err != nil {
				return err
			}
			if firstStage == nil {
				return apperr.New(
					apperr.Conflict,
					fmt.Sprintf("board %s has no stages", foundBoard.Title),
					nil,
				)
			}
			create.StageID = &firstStage.ID
		}

		created, err = store.Add(ctx, create, timestamp)
		if err != nil {
			return err
		}
		if len(fields.Tags) == 0 {
			return nil
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
	if options.AreaID == nil {
		return domain.NormalizeSliceResult(s.store.List(ctx, options))
	}

	var listed []Project
	err := s.store.WithinReadTransaction(ctx, func(transaction Transaction) error {
		if err := transaction.AreaExists(ctx, *options.AreaID); err != nil {
			return err
		}

		var err error
		listed, err = transaction.List(ctx, options)
		return err
	})
	if err != nil {
		return nil, err
	}

	return domain.NormalizeSliceResult(listed, nil)
}

func (s *Service) Show(ctx context.Context, id int64) (Detail, error) {
	if err := validateID(id); err != nil {
		return Detail{}, err
	}

	var detail Detail
	err := s.store.WithinReadTransaction(ctx, func(store Transaction) error {
		found, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		detail.Project = found
		if found.StageID == nil {
			return nil
		}
		stage, err := store.FindStageByID(ctx, *found.StageID)
		if err != nil {
			return err
		}
		detail.Location = locationForStage(stage)
		return nil
	})
	if err != nil {
		return Detail{}, err
	}
	return detail, nil
}

func (s *Service) Edit(ctx context.Context, id int64, fields EditFields) (Edition, error) {
	if err := validateID(id); err != nil {
		return Edition{}, err
	}
	if fields.Area.Set != nil && fields.Area.Clear {
		return Edition{}, apperr.New(
			apperr.InvalidArgument,
			"area cannot be set and cleared",
			nil,
		)
	}
	if fields.Board.Set != nil && fields.Board.Clear {
		return Edition{}, apperr.New(
			apperr.InvalidArgument,
			"board cannot be set and cleared",
			nil,
		)
	}
	if err := validateAreaID(fields.Area.Set); err != nil {
		return Edition{}, err
	}
	if fields.Board.Set != nil {
		if err := domain.ValidateTitle(*fields.Board.Set); err != nil {
			return Edition{}, err
		}
	}
	if fields.Title == nil && fields.Note == nil && fields.Area.Set == nil && !fields.Area.Clear &&
		fields.Board.Set == nil && !fields.Board.Clear {
		return Edition{}, apperr.New(
			apperr.InvalidArgument,
			"project edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := domain.ValidateTitle(*fields.Title); err != nil {
			return Edition{}, err
		}
	}
	if fields.Note != nil {
		if err := domain.ValidateNote(*fields.Note); err != nil {
			return Edition{}, err
		}
	}

	timestamp := domain.FormatTimestamp(s.now())
	result := Edition{ClearedDefers: []task.Task{}}
	if fields.Board.Set == nil && !fields.Board.Clear {
		updated := UpdateFields{Area: fields.Area, Title: fields.Title, Note: fields.Note}
		edited, err := s.store.Edit(ctx, id, updated, timestamp)
		if err != nil {
			return Edition{}, err
		}
		result.Project = edited
		return result, nil
	}

	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		current, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		updated := UpdateFields{Area: fields.Area, Title: fields.Title, Note: fields.Note}

		var currentStage *StageReference
		if current.StageID != nil {
			stage, err := store.FindStageByID(ctx, *current.StageID)
			if err != nil {
				return err
			}
			currentStage = &stage
		}

		boardMovement := false
		switch {
		case fields.Board.Set != nil:
			foundBoard, err := store.FindBoard(ctx, *fields.Board.Set)
			if err != nil {
				return err
			}
			if currentStage != nil && currentStage.BoardID == foundBoard.ID {
				result.Location = locationForStage(*currentStage)
				break
			}
			firstStage, err := store.FindFirstStage(ctx, foundBoard.ID)
			if err != nil {
				return err
			}
			if firstStage == nil {
				return apperr.New(
					apperr.Conflict,
					fmt.Sprintf("board %s has no stages", foundBoard.Title),
					nil,
				)
			}
			updated.Stage.Set = &firstStage.ID
			result.Location = locationForStage(*firstStage)
			boardMovement = true
		case fields.Board.Clear && currentStage != nil:
			updated.Stage.Clear = true
			boardMovement = true
		}

		areaReferences, areaMovement, err := resolveEditAreas(ctx, store, current, fields.Area)
		if err != nil {
			return err
		}
		if boardMovement || areaMovement {
			guardedProject := Project{}
			if boardMovement {
				guardedProject = current
			}
			if err := containmentConflict(
				fmt.Sprintf("move project %d", id),
				guardedProject,
				areaReferences...,
			); err != nil {
				return err
			}
		}

		if !hasUpdateFields(updated) {
			result.Project = current
			return nil
		}
		result.Project, err = store.Edit(ctx, id, updated, timestamp)
		return err
	})
	if err != nil {
		return Edition{}, err
	}
	return result, nil
}

func (s *Service) Move(
	ctx context.Context,
	id int64,
	stageTitle string,
	placement *domain.Placement,
) (Movement, error) {
	if err := validateID(id); err != nil {
		return Movement{}, err
	}
	if err := domain.ValidateTitle(stageTitle); err != nil {
		return Movement{}, err
	}
	if placement != nil {
		if err := domain.ValidatePlacement(*placement); err != nil {
			return Movement{}, err
		}
	}

	var result Movement
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		current, err := store.Find(ctx, id)
		if err != nil {
			return err
		}
		if current.StageID == nil {
			return apperr.New(
				apperr.Conflict,
				fmt.Sprintf("cannot move project %d while it is not on a board", id),
				nil,
			)
		}
		currentStage, err := store.FindStageByID(ctx, *current.StageID)
		if err != nil {
			return err
		}
		destination, err := store.FindStage(ctx, currentStage.BoardID, stageTitle)
		if err != nil {
			if code, coded := apperr.CodeOf(err); coded && code == apperr.NotFound {
				return apperr.New(
					apperr.NotFound,
					fmt.Sprintf("no stage %s on board %s", stageTitle, currentStage.BoardTitle),
					err,
				)
			}
			return err
		}
		if placement != nil &&
			(placement.Anchor == domain.PlacementAfter || placement.Anchor == domain.PlacementBefore) {
			reference, err := store.Find(ctx, placement.ReferenceID)
			if err != nil {
				return err
			}
			if reference.ID == current.ID {
				return apperr.New(
					apperr.InvalidArgument,
					"project cannot be placed relative to itself",
					nil,
				)
			}
			if reference.StageID == nil || *reference.StageID != destination.ID {
				return apperr.New(
					apperr.InvalidArgument,
					fmt.Sprintf("project %d is in a different stage", reference.ID),
					nil,
				)
			}
		}

		areaReferences := []AreaReference{}
		if current.AreaID != nil {
			foundArea, err := store.FindArea(ctx, *current.AreaID)
			if err != nil {
				return err
			}
			areaReferences = append(areaReferences, foundArea)
		}
		if err := containmentConflict(
			fmt.Sprintf("move project %d", id),
			current,
			areaReferences...,
		); err != nil {
			return err
		}

		result.StageTitle = destination.Title
		if destination.ID == currentStage.ID && placement == nil {
			result.Project = current
			return nil
		}
		numeric := domain.Placement{}
		if placement != nil {
			numeric = *placement
		}
		result.Project, err = store.MoveStage(
			ctx,
			id,
			destination.ID,
			numeric,
			domain.FormatTimestamp(s.now()),
		)
		return err
	})
	if err != nil {
		return Movement{}, err
	}
	return result, nil
}

func (s *Service) Reorder(ctx context.Context, id int64, placement domain.Placement) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}
	if err := domain.ValidatePlacement(placement); err != nil {
		return Project{}, err
	}

	return s.store.Reorder(ctx, id, placement, domain.FormatTimestamp(s.now()))
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

func resolveEditAreas(
	ctx context.Context,
	store Transaction,
	current Project,
	change AreaChange,
) ([]AreaReference, bool, error) {
	destination := current.AreaID
	var destinationArea *AreaReference
	if change.Set != nil {
		found, err := store.FindArea(ctx, *change.Set)
		if err != nil {
			return nil, false, err
		}
		destination = change.Set
		destinationArea = &found
	} else if change.Clear {
		destination = nil
	}

	movement := !sameOptionalID(current.AreaID, destination)
	references := make([]AreaReference, 0, 2)
	if current.AreaID != nil {
		if destinationArea != nil && destinationArea.ID == *current.AreaID {
			references = append(references, *destinationArea)
		} else {
			found, err := store.FindArea(ctx, *current.AreaID)
			if err != nil {
				return nil, false, err
			}
			references = append(references, found)
		}
	}
	if destinationArea != nil && (current.AreaID == nil || destinationArea.ID != *current.AreaID) {
		references = append(references, *destinationArea)
	}
	return references, movement, nil
}

func containmentConflict(action string, current Project, areas ...AreaReference) error {
	resolvedIDs := []int64{}
	if current.ID != 0 && (current.DoneAt != nil || current.CancelledAt != nil ||
		(current.Status != "" && current.Status != string(ListStatusOpen))) {
		resolvedIDs = append(resolvedIDs, current.ID)
	}
	archivedIDs := make([]int64, 0, len(areas))
	for _, currentArea := range areas {
		if currentArea.ArchivedAt != nil {
			archivedIDs = append(archivedIDs, currentArea.ID)
		}
	}
	if len(resolvedIDs) == 0 && len(archivedIDs) == 0 {
		return nil
	}

	slices.Sort(resolvedIDs)
	resolvedIDs = slices.Compact(resolvedIDs)
	slices.Sort(archivedIDs)
	archivedIDs = slices.Compact(archivedIDs)
	blockers := make([]string, 0, 2)
	causes := make([]error, 0, 2)
	if len(resolvedIDs) > 0 {
		blockers = append(blockers, fmt.Sprintf("project %d is resolved", resolvedIDs[0]))
		causes = append(causes, &ResolvedProjectsError{IDs: resolvedIDs})
	}
	if len(archivedIDs) == 1 {
		blockers = append(blockers, fmt.Sprintf("area %d is archived", archivedIDs[0]))
		causes = append(causes, &ArchivedAreasError{IDs: archivedIDs})
	} else if len(archivedIDs) > 1 {
		blockers = append(blockers, fmt.Sprintf("areas %v are archived", archivedIDs))
		causes = append(causes, &ArchivedAreasError{IDs: archivedIDs})
	}
	return apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot %s while %s", action, strings.Join(blockers, " and ")),
		errors.Join(causes...),
	)
}

func locationForStage(stage StageReference) *Location {
	return &Location{BoardTitle: stage.BoardTitle, StageTitle: stage.Title}
}

func hasUpdateFields(fields UpdateFields) bool {
	return fields.Title != nil || fields.Note != nil || fields.Area.Set != nil || fields.Area.Clear ||
		fields.Stage.Set != nil || fields.Stage.Clear
}

func sameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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
