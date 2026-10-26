package project

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

// Service validates project requests and orchestrates them against a Store.
type Service struct {
	store Store
	now   func() time.Time
}

// NewService returns a Service that stamps rows with the wall clock.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Add creates a project. An archived area is a conflict error, a named board
// must have a first stage, and tags are attached in the same transaction so
// an unknown tag creates nothing.
func (s *Service) Add(ctx context.Context, fields AddRequest) (Project, error) {
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
	create := AddFields{
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
			if err := containmentConflict("add project", nil, foundArea); err != nil {
				return err
			}
		}
		if fields.Board != nil {
			foundBoard, err := store.FindBoard(ctx, *fields.Board)
			if err != nil {
				return err
			}
			firstStage, err := requireFirstStage(ctx, store, foundBoard)
			if err != nil {
				return err
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

// List returns projects matching the status filter and, when set, the area,
// which must exist. The result is never nil on success.
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

// Show returns one project with its board location when it has one.
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

// Edit changes title, note, area, or board. Setting and clearing the same
// field, or changing nothing, is an invalid_argument error. Area and board
// moves are blocked by archived areas, board moves also by a resolved
// project, and a board move clears task defers that pointed at the old
// board's stages.
func (s *Service) Edit(ctx context.Context, id int64, fields EditRequest) (Edition, error) {
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
	boardRequested := fields.Board.Set != nil || fields.Board.Clear
	membershipRequested := fields.Area.Set != nil || fields.Area.Clear || boardRequested
	if !membershipRequested {
		updated := EditFields{Title: fields.Title, Note: fields.Note}
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
		updated := EditFields{Area: fields.Area, Title: fields.Title, Note: fields.Note}

		var currentStage *StageReference
		if boardRequested && current.StageID != nil {
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
			firstStage, err := requireFirstStage(ctx, store, foundBoard)
			if err != nil {
				return err
			}
			updated.Stage.Set = &firstStage.ID
			result.Location = locationForStage(firstStage)
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
			var resolvedGuard *Project
			if boardMovement {
				resolvedGuard = &current
			}
			if err := containmentConflict(
				fmt.Sprintf("move project %d", id),
				resolvedGuard,
				areaReferences...,
			); err != nil {
				return err
			}
		}
		if boardMovement {
			result.ClearedDefers, err = store.ClearTaskStageDefers(ctx, id, timestamp)
			if err != nil {
				return err
			}
		}

		if !hasEditFields(updated) {
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

// Move puts the project on the named stage of its current board, optionally
// at a placement among that stage's projects. A project not on a board is a
// conflict error, and a placement reference must be a different project in
// the destination stage.
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
					fmt.Sprintf("cannot move project %d relative to itself", id),
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
			&current,
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

// Reorder moves the project among its siblings in its container.
func (s *Service) Reorder(ctx context.Context, id int64, placement domain.Placement) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}
	if err := domain.ValidatePlacement(placement); err != nil {
		return Project{}, err
	}

	return s.store.Reorder(ctx, id, placement, domain.FormatTimestamp(s.now()))
}

// Resolve marks the project done or cancelled and cancels its open tasks in
// the same transaction.
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

// Reopen returns a resolved project to open; its tasks stay as they are.
func (s *Service) Reopen(ctx context.Context, id int64) (Project, error) {
	if err := validateID(id); err != nil {
		return Project{}, err
	}

	return s.store.Reopen(ctx, id, domain.FormatTimestamp(s.now()))
}

// Tag attaches existing tags to the project. Names already attached are
// left as they are, so repeating the call changes nothing.
func (s *Service) Tag(ctx context.Context, id int64, names []string) (Tagging, error) {
	return s.changeTags(ctx, id, names, true)
}

// Untag detaches tags from the project. Names not attached are left alone.
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

// Delete removes the project. Without recursive the store refuses a project
// that still has tasks; with it the tasks are deleted in the same
// transaction and reported.
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

	movement := !domain.SameOptionalID(current.AreaID, destination)
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

// resolvedGuard is nil when the operation permits resolved projects (area
// re-filing); board-axis movement passes the project to refuse resolved ones.
func containmentConflict(action string, resolvedGuard *Project, areas ...AreaReference) error {
	resolved := resolvedGuard != nil && (resolvedGuard.DoneAt != nil || resolvedGuard.CancelledAt != nil ||
		(resolvedGuard.Status != "" && resolvedGuard.Status != string(ListStatusOpen)))
	archivedIDs := make([]int64, 0, len(areas))
	for _, currentArea := range areas {
		if currentArea.ArchivedAt != nil {
			archivedIDs = append(archivedIDs, currentArea.ID)
		}
	}
	if !resolved && len(archivedIDs) == 0 {
		return nil
	}

	archivedIDs = domain.SortedUniqueIDs(archivedIDs)
	blockers := make([]string, 0, 2)
	causes := make([]error, 0, 2)
	if resolved {
		blockers = append(blockers, fmt.Sprintf("project %d is resolved", resolvedGuard.ID))
		causes = append(causes, &ResolvedProjectsError{IDs: []int64{resolvedGuard.ID}})
	}
	if len(archivedIDs) == 1 {
		blockers = append(blockers, fmt.Sprintf("area %d is archived", archivedIDs[0]))
		causes = append(causes, &ArchivedAreasError{IDs: archivedIDs})
	} else if len(archivedIDs) > 1 {
		blockers = append(blockers, fmt.Sprintf("areas %s are archived", domain.FormatIDs(archivedIDs)))
		causes = append(causes, &ArchivedAreasError{IDs: archivedIDs})
	}
	return apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot %s while %s", action, strings.Join(blockers, " and ")),
		errors.Join(causes...),
	)
}

func requireFirstStage(
	ctx context.Context,
	store Transaction,
	currentBoard BoardReference,
) (StageReference, error) {
	stage, err := store.FindFirstStage(ctx, currentBoard.ID)
	if err != nil {
		return StageReference{}, err
	}
	if stage == nil {
		return StageReference{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("board %s has no stages", currentBoard.Title),
			nil,
		)
	}
	return *stage, nil
}

func locationForStage(stage StageReference) *Location {
	return &Location{BoardTitle: stage.BoardTitle, StageTitle: stage.Title}
}

func hasEditFields(fields EditFields) bool {
	return fields.Title != nil || fields.Note != nil || fields.Area.Set != nil || fields.Area.Clear ||
		fields.Stage.Set != nil || fields.Stage.Clear
}

// ParseID parses a project ID argument with domain.ParseID semantics.
func ParseID(value string) (int64, error) {
	return domain.ParseID("project", value)
}

// ParseListStatus accepts open, done, cancelled, or all; anything else is an
// invalid_argument error.
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
