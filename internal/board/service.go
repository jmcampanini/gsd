package board

import (
	"context"
	"fmt"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, fields AddFields) (Addition, error) {
	if err := domain.ValidateTitle(fields.Title); err != nil {
		return Addition{}, err
	}
	if err := domain.ValidateNote(fields.Note); err != nil {
		return Addition{}, err
	}
	if len(fields.Stages) == 0 {
		return Addition{}, apperr.New(
			apperr.InvalidArgument,
			"board add requires at least one stage",
			nil,
		)
	}
	for _, title := range fields.Stages {
		if err := domain.ValidateTitle(title); err != nil {
			return Addition{}, err
		}
	}

	timestamp := domain.FormatTimestamp(s.now())
	result := Addition{Stages: []Stage{}}
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		added, err := store.AddBoard(ctx, fields, timestamp)
		if err != nil {
			return err
		}
		result.Board = added

		for _, title := range fields.Stages {
			stage, err := store.AddStage(ctx, added.ID, title, timestamp)
			if err != nil {
				return err
			}
			result.Stages = append(result.Stages, stage)
		}
		return nil
	})
	if err != nil {
		return Addition{}, err
	}
	return result, nil
}

func (s *Service) List(ctx context.Context) ([]ListedBoard, error) {
	listed := []ListedBoard{}
	err := s.store.WithinReadTransaction(ctx, func(store Transaction) error {
		boards, err := domain.NormalizeSliceResult(store.ListBoards(ctx))
		if err != nil {
			return err
		}

		for _, current := range boards {
			stages, err := domain.NormalizeSliceResult(store.ListStages(ctx, current.ID))
			if err != nil {
				return err
			}
			listed = append(listed, ListedBoard{Board: current, Stages: stages})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return listed, nil
}

func (s *Service) Show(ctx context.Context, title string) (Show, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return Show{}, err
	}

	return s.show(ctx, func(store Transaction) (Board, error) {
		return store.FindBoard(ctx, title)
	})
}

func (s *Service) ShowByID(ctx context.Context, id int64) (Show, error) {
	if err := domain.ValidateID("board", id); err != nil {
		return Show{}, err
	}

	return s.show(ctx, func(store Transaction) (Board, error) {
		return store.FindBoardByID(ctx, id)
	})
}

func (s *Service) show(
	ctx context.Context,
	find func(Transaction) (Board, error),
) (Show, error) {
	result := Show{Stages: []ShownStage{}}
	err := s.store.WithinReadTransaction(ctx, func(store Transaction) error {
		found, err := find(store)
		if err != nil {
			return err
		}
		stages, err := domain.NormalizeSliceResult(store.ListStages(ctx, found.ID))
		if err != nil {
			return err
		}
		projects, err := domain.NormalizeSliceResult(store.ListShownProjects(ctx, found.ID))
		if err != nil {
			return err
		}

		result.Board = found
		stageIndexes := make(map[int64]int, len(stages))
		for _, stage := range stages {
			stageIndexes[stage.ID] = len(result.Stages)
			result.Stages = append(result.Stages, ShownStage{
				Stage:    stage,
				Projects: []ShownProject{},
			})
		}
		for _, current := range projects {
			if current.StageID == nil {
				return apperr.New(
					apperr.Internal,
					fmt.Sprintf("shown project %d has no stage", current.ID),
					nil,
				)
			}
			index, exists := stageIndexes[*current.StageID]
			if !exists {
				return apperr.New(
					apperr.Internal,
					fmt.Sprintf("shown project %d has stage outside board", current.ID),
					nil,
				)
			}
			result.Stages[index].Projects = append(result.Stages[index].Projects, current)
		}
		return nil
	})
	if err != nil {
		return Show{}, err
	}
	return result, nil
}

func (s *Service) Edit(ctx context.Context, title string, fields EditFields) (Board, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return Board{}, err
	}
	if fields.Title == nil && fields.Note == nil {
		return Board{}, apperr.New(
			apperr.InvalidArgument,
			"board edit requires at least one field",
			nil,
		)
	}
	if fields.Title != nil {
		if err := domain.ValidateTitle(*fields.Title); err != nil {
			return Board{}, err
		}
	}
	if fields.Note != nil {
		if err := domain.ValidateNote(*fields.Note); err != nil {
			return Board{}, err
		}
	}

	timestamp := domain.FormatTimestamp(s.now())
	var edited Board
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		found, err := store.FindBoard(ctx, title)
		if err != nil {
			return err
		}
		edited, err = store.EditBoard(ctx, found.ID, fields, timestamp)
		return err
	})
	if err != nil {
		return Board{}, err
	}
	return edited, nil
}

func (s *Service) Reorder(ctx context.Context, title string, placement Placement) (Board, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return Board{}, err
	}
	if err := validatePlacement(placement); err != nil {
		return Board{}, err
	}

	timestamp := domain.FormatTimestamp(s.now())
	var reordered Board
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		found, err := store.FindBoard(ctx, title)
		if err != nil {
			return err
		}
		numeric, err := resolveBoardPlacement(ctx, store, found, placement)
		if err != nil {
			return err
		}
		reordered, err = store.ReorderBoard(ctx, found.ID, numeric, timestamp)
		return err
	})
	if err != nil {
		return Board{}, err
	}
	return reordered, nil
}

func (s *Service) Delete(ctx context.Context, title string) (Deletion, error) {
	if err := domain.ValidateTitle(title); err != nil {
		return Deletion{}, err
	}

	result := Deletion{Stages: []Stage{}}
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		found, err := store.FindBoard(ctx, title)
		if err != nil {
			return err
		}
		occupancy, err := store.BoardOccupancy(ctx, found.ID)
		if err != nil {
			return err
		}
		if occupancy.Any() {
			return apperr.New(
				apperr.Conflict,
				fmt.Sprintf(
					"cannot delete board %s while it contains %s",
					found.Title,
					occupancyPhrase(occupancy),
				),
				nil,
			)
		}
		stages, err := domain.NormalizeSliceResult(store.ListStages(ctx, found.ID))
		if err != nil {
			return err
		}
		deleted, err := store.DeleteBoard(ctx, found.ID)
		if err != nil {
			return err
		}
		result = Deletion{Board: deleted, Stages: stages}
		return nil
	})
	if err != nil {
		return Deletion{}, err
	}
	return result, nil
}

func (s *Service) AddStage(
	ctx context.Context,
	boardTitle string,
	stageTitle string,
	placement *Placement,
) (StageResult, error) {
	if err := validateBoardAndStageTitles(boardTitle, stageTitle); err != nil {
		return StageResult{}, err
	}
	if placement != nil {
		if err := validatePlacement(*placement); err != nil {
			return StageResult{}, err
		}
	}

	timestamp := domain.FormatTimestamp(s.now())
	appendLast := placement == nil || placement.Anchor == domain.PlacementLast
	var result StageResult
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		foundBoard, err := store.FindBoard(ctx, boardTitle)
		if err != nil {
			return err
		}
		var numeric domain.Placement
		if !appendLast {
			numeric, err = resolveStagePlacement(ctx, store, foundBoard, *placement)
			if err != nil {
				return err
			}
		}
		added, err := store.AddStage(ctx, foundBoard.ID, stageTitle, timestamp)
		if err != nil {
			return err
		}

		result = StageResult{Board: foundBoard, Stage: added}
		if appendLast {
			return nil
		}
		result.Stage, err = store.ReorderStage(ctx, foundBoard.ID, added.ID, numeric, timestamp)
		return err
	})
	if err != nil {
		return StageResult{}, err
	}
	return result, nil
}

func (s *Service) RenameStage(
	ctx context.Context,
	boardTitle string,
	stageTitle string,
	newTitle string,
) (StageRenameResult, error) {
	if err := validateBoardAndStageTitles(boardTitle, stageTitle); err != nil {
		return StageRenameResult{}, err
	}
	if err := domain.ValidateTitle(newTitle); err != nil {
		return StageRenameResult{}, err
	}

	timestamp := domain.FormatTimestamp(s.now())
	var result StageRenameResult
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		foundBoard, err := store.FindBoard(ctx, boardTitle)
		if err != nil {
			return err
		}
		foundStage, err := store.FindStage(ctx, foundBoard.ID, stageTitle)
		if err != nil {
			return err
		}
		renamed, err := store.RenameStage(
			ctx,
			foundBoard.ID,
			foundStage.ID,
			newTitle,
			timestamp,
		)
		if err != nil {
			return err
		}
		result = StageRenameResult{
			Board:         foundBoard,
			Stage:         renamed,
			PreviousTitle: foundStage.Title,
		}
		return nil
	})
	if err != nil {
		return StageRenameResult{}, err
	}
	return result, nil
}

func (s *Service) ReorderStage(
	ctx context.Context,
	boardTitle string,
	stageTitle string,
	placement Placement,
) (StageResult, error) {
	if err := validateBoardAndStageTitles(boardTitle, stageTitle); err != nil {
		return StageResult{}, err
	}
	if err := validatePlacement(placement); err != nil {
		return StageResult{}, err
	}

	timestamp := domain.FormatTimestamp(s.now())
	var result StageResult
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		foundBoard, err := store.FindBoard(ctx, boardTitle)
		if err != nil {
			return err
		}
		foundStage, err := store.FindStage(ctx, foundBoard.ID, stageTitle)
		if err != nil {
			return err
		}
		numeric, err := resolveStagePlacement(ctx, store, foundBoard, placement)
		if err != nil {
			return err
		}
		if (placement.Anchor == domain.PlacementAfter || placement.Anchor == domain.PlacementBefore) &&
			numeric.ReferenceID == foundStage.ID {
			return apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("cannot reorder stage %s relative to itself", foundStage.Title),
				nil,
			)
		}
		reordered, err := store.ReorderStage(
			ctx,
			foundBoard.ID,
			foundStage.ID,
			numeric,
			timestamp,
		)
		if err != nil {
			return err
		}
		result = StageResult{Board: foundBoard, Stage: reordered}
		return nil
	})
	if err != nil {
		return StageResult{}, err
	}
	return result, nil
}

func (s *Service) DeleteStage(
	ctx context.Context,
	boardTitle string,
	stageTitle string,
) (StageDeletion, error) {
	if err := validateBoardAndStageTitles(boardTitle, stageTitle); err != nil {
		return StageDeletion{}, err
	}

	timestamp := domain.FormatTimestamp(s.now())
	var result StageDeletion
	err := s.store.WithinTransaction(ctx, func(store Transaction) error {
		foundBoard, err := store.FindBoard(ctx, boardTitle)
		if err != nil {
			return err
		}
		foundStage, err := store.FindStage(ctx, foundBoard.ID, stageTitle)
		if err != nil {
			return err
		}
		occupancy, err := store.StageOccupancy(ctx, foundStage.ID)
		if err != nil {
			return err
		}
		if occupancy.Any() {
			return apperr.New(
				apperr.Conflict,
				fmt.Sprintf(
					"cannot delete stage %s on board %s while it contains %s",
					foundStage.Title,
					foundBoard.Title,
					occupancyPhrase(occupancy),
				),
				nil,
			)
		}
		cleared, err := store.ClearTaskStageDefers(ctx, foundStage.ID, timestamp)
		if err != nil {
			return err
		}
		deleted, err := store.DeleteStage(ctx, foundBoard.ID, foundStage.ID)
		if err != nil {
			return err
		}
		result = StageDeletion{Board: foundBoard, Stage: deleted, ClearedDefers: cleared}
		return nil
	})
	if err != nil {
		return StageDeletion{}, err
	}
	return result, nil
}

func resolveBoardPlacement(
	ctx context.Context,
	store Transaction,
	subject Board,
	placement Placement,
) (domain.Placement, error) {
	numeric := domain.Placement{Anchor: placement.Anchor}
	if placement.Anchor != domain.PlacementAfter && placement.Anchor != domain.PlacementBefore {
		return numeric, nil
	}

	reference, err := store.FindBoard(ctx, placement.Reference)
	if err != nil {
		return domain.Placement{}, err
	}
	if reference.ID == subject.ID {
		return domain.Placement{}, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("cannot reorder board %s relative to itself", subject.Title),
			nil,
		)
	}
	numeric.ReferenceID = reference.ID
	return numeric, nil
}

func resolveStagePlacement(
	ctx context.Context,
	store Transaction,
	currentBoard Board,
	placement Placement,
) (domain.Placement, error) {
	numeric := domain.Placement{Anchor: placement.Anchor}
	if placement.Anchor != domain.PlacementAfter && placement.Anchor != domain.PlacementBefore {
		return numeric, nil
	}

	reference, err := store.FindStage(ctx, currentBoard.ID, placement.Reference)
	if err != nil {
		return domain.Placement{}, err
	}
	numeric.ReferenceID = reference.ID
	return numeric, nil
}

func validateBoardAndStageTitles(boardTitle, stageTitle string) error {
	if err := domain.ValidateTitle(boardTitle); err != nil {
		return err
	}
	return domain.ValidateTitle(stageTitle)
}

func validatePlacement(placement Placement) error {
	return domain.ValidateNamedPlacement(placement.Anchor, placement.Reference)
}

func occupancyPhrase(occupancy Occupancy) string {
	switch {
	case occupancy.Open > 0 && occupancy.Resolved > 0:
		return fmt.Sprintf("projects (%d open, %d resolved)", occupancy.Open, occupancy.Resolved)
	case occupancy.Resolved > 0:
		return fmt.Sprintf("projects (%d resolved)", occupancy.Resolved)
	default:
		return "projects"
	}
}
