package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/task"
)

const (
	boardColumns          = `id, title, note, position, created_at, updated_at`
	stageColumns          = `id, board_id, title, position, created_at, updated_at`
	stageWithBoardColumns = `s.id, s.board_id, s.title, s.position, s.created_at, s.updated_at, b.title`
)

type Boards struct {
	database *DB
}

type boardsCore struct {
	executor boardExecutor
}

type boardExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func NewBoards(database *DB) *Boards {
	return &Boards{database: database}
}

func (s *Boards) AddBoard(
	ctx context.Context,
	fields board.AddFields,
	timestamp string,
) (board.Board, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Board, error) {
		return transaction.AddBoard(ctx, fields, timestamp)
	})
}

func (s *Boards) FindBoard(ctx context.Context, name string) (board.Board, error) {
	return s.poolCore().FindBoard(ctx, name)
}

func (s *Boards) ListBoards(ctx context.Context) ([]board.Board, error) {
	return s.poolCore().ListBoards(ctx)
}

func (s *Boards) EditBoard(
	ctx context.Context,
	id int64,
	fields board.EditFields,
	timestamp string,
) (board.Board, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Board, error) {
		return transaction.EditBoard(ctx, id, fields, timestamp)
	})
}

func (s *Boards) ReorderBoard(
	ctx context.Context,
	id int64,
	placement domain.Placement,
	timestamp string,
) (board.Board, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Board, error) {
		return transaction.ReorderBoard(ctx, id, placement, timestamp)
	})
}

func (s *Boards) DeleteBoard(ctx context.Context, id int64) (board.Board, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Board, error) {
		return transaction.DeleteBoard(ctx, id)
	})
}

func (s *Boards) AddStage(
	ctx context.Context,
	boardID int64,
	title, timestamp string,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.AddStage(ctx, boardID, title, timestamp)
	})
}

func (s *Boards) FindStage(ctx context.Context, boardID int64, name string) (board.Stage, error) {
	return s.poolCore().FindStage(ctx, boardID, name)
}

func (s *Boards) ListStages(ctx context.Context, boardID int64) ([]board.Stage, error) {
	return s.poolCore().ListStages(ctx, boardID)
}

func (s *Boards) ListShownProjects(
	ctx context.Context,
	boardID int64,
) ([]board.ShownProject, error) {
	return s.poolCore().ListShownProjects(ctx, boardID)
}

func (s *Boards) BoardOccupancy(ctx context.Context, boardID int64) (board.Occupancy, error) {
	return s.poolCore().BoardOccupancy(ctx, boardID)
}

func (s *Boards) StageOccupancy(ctx context.Context, stageID int64) (board.Occupancy, error) {
	return s.poolCore().StageOccupancy(ctx, stageID)
}

func (s *Boards) ClearTaskStageDefers(
	ctx context.Context,
	stageID int64,
	timestamp string,
) ([]task.Task, error) {
	return s.poolCore().ClearTaskStageDefers(ctx, stageID, timestamp)
}

func (s *Boards) RenameStage(
	ctx context.Context,
	boardID, id int64,
	newName, timestamp string,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.RenameStage(ctx, boardID, id, newName, timestamp)
	})
}

func (s *Boards) ReorderStage(
	ctx context.Context,
	boardID, id int64,
	placement domain.Placement,
	timestamp string,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.ReorderStage(ctx, boardID, id, placement, timestamp)
	})
}

func (s *Boards) DeleteStage(
	ctx context.Context,
	boardID, id int64,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.DeleteStage(ctx, boardID, id)
	})
}

func (s *Boards) WithinTransaction(
	ctx context.Context,
	apply func(board.Transaction) error,
) error {
	return withinImmediateTransaction(ctx, s.database, "board", func(connection *sql.Conn) error {
		return apply(&boardsCore{executor: connection})
	})
}

func (s *Boards) WithinReadTransaction(
	ctx context.Context,
	apply func(board.Transaction) error,
) error {
	return withinDeferredTransaction(ctx, s.database, "board", func(connection *sql.Conn) error {
		return apply(&boardsCore{executor: connection})
	})
}

func (s *Boards) poolCore() *boardsCore {
	return &boardsCore{executor: s.database.database}
}

func (s *boardsCore) AddBoard(
	ctx context.Context,
	fields board.AddFields,
	timestamp string,
) (board.Board, error) {
	created, err := scanBoard(s.executor.QueryRowContext(ctx, `
INSERT INTO boards (title, note, position, created_at, updated_at)
SELECT ?, ?, COALESCE((SELECT MAX(position) FROM boards), -1) + 1, ?, ?
WHERE NOT EXISTS (SELECT 1 FROM boards WHERE title = ? COLLATE NOCASE)
RETURNING `+boardColumns, fields.Title, fields.Note, timestamp, timestamp, fields.Title))
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return board.Board{}, fmt.Errorf("insert board: %w", err)
	}

	existing, findErr := s.FindBoard(ctx, fields.Title)
	if findErr != nil {
		return board.Board{}, fmt.Errorf("classify board insert: %w", errors.Join(err, findErr))
	}
	return board.Board{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("board already exists: %s", existing.Title),
		err,
	)
}

func (s *boardsCore) FindBoard(ctx context.Context, name string) (board.Board, error) {
	found, err := scanBoard(s.executor.QueryRowContext(
		ctx,
		"SELECT "+boardColumns+" FROM boards WHERE title = ? COLLATE NOCASE",
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Board{}, apperr.New(apperr.NotFound, fmt.Sprintf("no board %s", name), err)
	}
	if err != nil {
		return board.Board{}, fmt.Errorf("find board: %w", err)
	}
	return found, nil
}

func (s *boardsCore) findBoardByID(ctx context.Context, id int64) (board.Board, error) {
	found, err := scanBoard(s.executor.QueryRowContext(
		ctx,
		"SELECT "+boardColumns+" FROM boards WHERE id = ?",
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Board{}, apperr.New(apperr.NotFound, fmt.Sprintf("no board %d", id), err)
	}
	if err != nil {
		return board.Board{}, fmt.Errorf("find board by ID: %w", err)
	}
	return found, nil
}

func (s *boardsCore) ListBoards(ctx context.Context) ([]board.Board, error) {
	rows, err := s.executor.QueryContext(ctx, "SELECT "+boardColumns+" FROM boards ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("list boards: %w", err)
	}
	return collectRows(rows, scanBoard, "scan listed board", "iterate listed boards")
}

func (s *boardsCore) EditBoard(
	ctx context.Context,
	id int64,
	fields board.EditFields,
	timestamp string,
) (board.Board, error) {
	if fields.Title == nil && fields.Note == nil {
		return board.Board{}, errors.New("board edit requires at least one field")
	}

	assignments := make([]string, 0, 3)
	arguments := make([]any, 0, 5)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}
	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, id)

	query := "UPDATE boards SET " + strings.Join(assignments, ", ") + " WHERE id = ?"
	if fields.Title != nil {
		query += `
  AND NOT EXISTS (
      SELECT 1 FROM boards AS existing
      WHERE existing.title = ? COLLATE NOCASE AND existing.id != boards.id
  )`
		arguments = append(arguments, *fields.Title)
	}

	edited, err := scanBoard(s.executor.QueryRowContext(ctx, query+" RETURNING "+boardColumns, arguments...))
	if err == nil {
		return edited, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return board.Board{}, fmt.Errorf("edit board: %w", err)
	}

	if _, findErr := s.findBoardByID(ctx, id); findErr != nil {
		return board.Board{}, findErr
	}
	if fields.Title == nil {
		return board.Board{}, fmt.Errorf("edit board: %w", err)
	}
	existing, findErr := s.FindBoard(ctx, *fields.Title)
	if findErr != nil {
		return board.Board{}, fmt.Errorf("classify board edit: %w", errors.Join(err, findErr))
	}
	return board.Board{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("board already exists: %s", existing.Title),
		err,
	)
}

func (s *boardsCore) ReorderBoard(
	ctx context.Context,
	id int64,
	placement domain.Placement,
	timestamp string,
) (board.Board, error) {
	if _, err := s.findBoardByID(ctx, id); err != nil {
		return board.Board{}, err
	}
	if placement.Anchor == domain.PlacementAfter || placement.Anchor == domain.PlacementBefore {
		if _, err := s.findBoardByID(ctx, placement.ReferenceID); err != nil {
			return board.Board{}, err
		}
		if id == placement.ReferenceID {
			return board.Board{}, apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("cannot reorder board %d relative to itself", id),
				nil,
			)
		}
	}

	rows, err := s.executor.QueryContext(ctx, "SELECT id FROM boards ORDER BY position, id")
	if err != nil {
		return board.Board{}, fmt.Errorf("list board reorder siblings: %w", err)
	}
	ordered, err := collectSiblingIDs(rows, "board")
	if err != nil {
		return board.Board{}, err
	}
	ordered, err = spliceOrderedIDs(ordered, id, placement)
	if err != nil {
		return board.Board{}, fmt.Errorf("reorder board positions: %w", err)
	}
	clause, arguments := reorderCaseUpdate(ordered, id, timestamp)
	if _, err := s.executor.ExecContext(ctx, "UPDATE boards SET "+clause, arguments...); err != nil {
		return board.Board{}, fmt.Errorf("reorder board: %w", err)
	}
	return s.findBoardByID(ctx, id)
}

func (s *boardsCore) DeleteBoard(ctx context.Context, id int64) (board.Board, error) {
	deleted, err := s.findBoardByID(ctx, id)
	if err != nil {
		return board.Board{}, err
	}
	if err := deleteRows(ctx, s.executor, 1, "DELETE FROM boards WHERE id = ?", id); err != nil {
		return board.Board{}, fmt.Errorf("delete board: %w", err)
	}
	return deleted, nil
}

func (s *boardsCore) AddStage(
	ctx context.Context,
	boardID int64,
	title, timestamp string,
) (board.Stage, error) {
	if _, err := s.findBoardByID(ctx, boardID); err != nil {
		return board.Stage{}, err
	}
	created, err := scanStage(s.executor.QueryRowContext(ctx, `
INSERT INTO stages (board_id, title, position, created_at, updated_at)
SELECT ?, ?, COALESCE((SELECT MAX(position) FROM stages WHERE board_id = ?), -1) + 1, ?, ?
WHERE NOT EXISTS (
    SELECT 1 FROM stages WHERE board_id = ? AND title = ? COLLATE NOCASE
)
RETURNING `+stageColumns, boardID, title, boardID, timestamp, timestamp, boardID, title))
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, fmt.Errorf("insert stage: %w", err)
	}

	existing, findErr := s.FindStage(ctx, boardID, title)
	if findErr != nil {
		return board.Stage{}, fmt.Errorf("classify stage insert: %w", errors.Join(err, findErr))
	}
	return board.Stage{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("stage already exists: %s", existing.Title),
		err,
	)
}

func (s *boardsCore) FindStage(ctx context.Context, boardID int64, name string) (board.Stage, error) {
	found, _, err := s.findStageWithBoard(ctx, boardID, name)
	return found, err
}

func (s *boardsCore) findStageWithBoard(
	ctx context.Context,
	boardID int64,
	name string,
) (board.Stage, string, error) {
	found, boardTitle, err := scanStageWithBoard(s.executor.QueryRowContext(ctx, `
SELECT `+stageWithBoardColumns+`
FROM stages s
JOIN boards b ON b.id = s.board_id
WHERE s.board_id = ? AND s.title = ? COLLATE NOCASE`, boardID, name))
	if errors.Is(err, sql.ErrNoRows) {
		owner, findErr := s.findBoardByID(ctx, boardID)
		if findErr != nil {
			return board.Stage{}, "", findErr
		}
		return board.Stage{}, "", apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no stage %s on board %s", name, owner.Title),
			err,
		)
	}
	if err != nil {
		return board.Stage{}, "", fmt.Errorf("find stage: %w", err)
	}
	return found, boardTitle, nil
}

func (s *boardsCore) findStageByIDWithBoard(
	ctx context.Context,
	id int64,
) (board.Stage, string, error) {
	found, boardTitle, err := scanStageWithBoard(s.executor.QueryRowContext(ctx, `
SELECT `+stageWithBoardColumns+`
FROM stages s
JOIN boards b ON b.id = s.board_id
WHERE s.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, "", apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no stage %d", id),
			err,
		)
	}
	if err != nil {
		return board.Stage{}, "", fmt.Errorf("find stage by ID: %w", err)
	}
	return found, boardTitle, nil
}

func (s *boardsCore) findFirstStageWithBoard(
	ctx context.Context,
	boardID int64,
) (*board.Stage, string, error) {
	found, boardTitle, err := scanStageWithBoard(s.executor.QueryRowContext(ctx, `
SELECT `+stageWithBoardColumns+`
FROM stages s
JOIN boards b ON b.id = s.board_id
WHERE s.board_id = ?
ORDER BY s.position, s.id
LIMIT 1`, boardID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("find first stage: %w", err)
	}
	return &found, boardTitle, nil
}

func (s *boardsCore) findStageByID(ctx context.Context, boardID, id int64) (board.Stage, error) {
	found, err := scanStage(s.executor.QueryRowContext(
		ctx,
		"SELECT "+stageColumns+" FROM stages WHERE board_id = ? AND id = ?",
		boardID,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no stage %d on board %d", id, boardID),
			err,
		)
	}
	if err != nil {
		return board.Stage{}, fmt.Errorf("find stage by ID: %w", err)
	}
	return found, nil
}

func (s *boardsCore) ListStages(ctx context.Context, boardID int64) ([]board.Stage, error) {
	rows, err := s.executor.QueryContext(
		ctx,
		"SELECT "+stageColumns+" FROM stages WHERE board_id = ? ORDER BY position, id",
		boardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list stages: %w", err)
	}
	return collectRows(rows, scanStage, "scan listed stage", "iterate listed stages")
}

func (s *boardsCore) ListShownProjects(
	ctx context.Context,
	boardID int64,
) ([]board.ShownProject, error) {
	rows, err := s.executor.QueryContext(ctx, `
SELECT `+qualifiedColumns("p", projectColumns)+`,
       `+tagJSONExpression(projectTagSpec, "p.id")+` AS tags,
       (SELECT COUNT(*)
        FROM tasks done
        WHERE done.project_id = p.id AND done.done_at IS NOT NULL) AS done_count,
       (SELECT COUNT(*)
        FROM tasks counted
        WHERE counted.project_id = p.id AND counted.cancelled_at IS NULL) AS total_count
FROM projects p
JOIN stages s ON s.id = p.stage_id
WHERE s.board_id = ? AND p.status = 'open'
ORDER BY s.position, s.id, p.stage_position, p.id`, boardID)
	if err != nil {
		return nil, fmt.Errorf("list shown board projects: %w", err)
	}
	return collectRows(
		rows,
		scanShownProject,
		"scan shown board project",
		"iterate shown board projects",
	)
}

func (s *boardsCore) BoardOccupancy(ctx context.Context, boardID int64) (board.Occupancy, error) {
	var occupancy board.Occupancy
	if err := s.executor.QueryRowContext(ctx, `
SELECT COUNT(*) FILTER (WHERE p.status = 'open'),
       COUNT(*) FILTER (WHERE p.status != 'open')
FROM projects p
JOIN stages s ON s.id = p.stage_id
WHERE s.board_id = ?`, boardID).Scan(&occupancy.Open, &occupancy.Resolved); err != nil {
		return board.Occupancy{}, fmt.Errorf("check board occupancy: %w", err)
	}
	return occupancy, nil
}

func (s *boardsCore) StageOccupancy(ctx context.Context, stageID int64) (board.Occupancy, error) {
	var occupancy board.Occupancy
	if err := s.executor.QueryRowContext(ctx, `
SELECT COUNT(*) FILTER (WHERE status = 'open'),
       COUNT(*) FILTER (WHERE status != 'open')
FROM projects
WHERE stage_id = ?`, stageID).Scan(&occupancy.Open, &occupancy.Resolved); err != nil {
		return board.Occupancy{}, fmt.Errorf("check stage occupancy: %w", err)
	}
	return occupancy, nil
}

func (s *boardsCore) ClearTaskStageDefers(
	ctx context.Context,
	stageID int64,
	timestamp string,
) ([]task.Task, error) {
	rows, err := s.executor.QueryContext(ctx, `
UPDATE tasks
SET defer_stage_id = NULL, updated_at = ?
WHERE defer_stage_id = ?
RETURNING `+taskColumnsWithTags("tasks.id"), timestamp, stageID)
	if err != nil {
		return nil, fmt.Errorf("clear stage task defers: %w", err)
	}

	cleared, err := collectRows(rows, scanTask, "scan cleared stage task defer", "iterate cleared stage task defers")
	if err != nil {
		return nil, err
	}
	sortTasks(cleared)
	return cleared, nil
}

func (s *boardsCore) RenameStage(
	ctx context.Context,
	boardID, id int64,
	newName, timestamp string,
) (board.Stage, error) {
	renamed, err := scanStage(s.executor.QueryRowContext(ctx, `
UPDATE stages
SET title = ?, updated_at = ?
WHERE board_id = ? AND id = ?
  AND NOT EXISTS (
      SELECT 1 FROM stages AS existing
      WHERE existing.board_id = stages.board_id
        AND existing.title = ? COLLATE NOCASE
        AND existing.id != stages.id
  )
RETURNING `+stageColumns, newName, timestamp, boardID, id, newName))
	if err == nil {
		return renamed, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, fmt.Errorf("rename stage: %w", err)
	}

	if _, findErr := s.findStageByID(ctx, boardID, id); findErr != nil {
		return board.Stage{}, findErr
	}
	existing, findErr := s.FindStage(ctx, boardID, newName)
	if findErr != nil {
		return board.Stage{}, fmt.Errorf("classify stage rename: %w", errors.Join(err, findErr))
	}
	return board.Stage{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("stage already exists: %s", existing.Title),
		err,
	)
}

func (s *boardsCore) ReorderStage(
	ctx context.Context,
	boardID, id int64,
	placement domain.Placement,
	timestamp string,
) (board.Stage, error) {
	if _, err := s.findStageByID(ctx, boardID, id); err != nil {
		return board.Stage{}, err
	}
	if placement.Anchor == domain.PlacementAfter || placement.Anchor == domain.PlacementBefore {
		if _, err := s.findStageByID(ctx, boardID, placement.ReferenceID); err != nil {
			return board.Stage{}, err
		}
		if id == placement.ReferenceID {
			return board.Stage{}, apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("cannot reorder stage %d relative to itself", id),
				nil,
			)
		}
	}

	rows, err := s.executor.QueryContext(ctx, `
SELECT id FROM stages WHERE board_id = ? ORDER BY position, id`, boardID)
	if err != nil {
		return board.Stage{}, fmt.Errorf("list stage reorder siblings: %w", err)
	}
	ordered, err := collectSiblingIDs(rows, "stage")
	if err != nil {
		return board.Stage{}, err
	}
	ordered, err = spliceOrderedIDs(ordered, id, placement)
	if err != nil {
		return board.Stage{}, fmt.Errorf("reorder stage positions: %w", err)
	}
	clause, arguments := reorderCaseUpdate(ordered, id, timestamp)
	if _, err := s.executor.ExecContext(ctx, "UPDATE stages SET "+clause, arguments...); err != nil {
		return board.Stage{}, fmt.Errorf("reorder stage: %w", err)
	}
	return s.findStageByID(ctx, boardID, id)
}

func (s *boardsCore) DeleteStage(
	ctx context.Context,
	boardID, id int64,
) (board.Stage, error) {
	deleted, err := scanStage(s.executor.QueryRowContext(
		ctx,
		"DELETE FROM stages WHERE board_id = ? AND id = ? RETURNING "+stageColumns,
		boardID,
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no stage %d on board %d", id, boardID),
			err,
		)
	}
	if err != nil {
		return board.Stage{}, fmt.Errorf("delete stage: %w", err)
	}
	return deleted, nil
}

func scanBoard(scanner rowScanner) (board.Board, error) {
	var value board.Board
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.Note,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}

func scanStage(scanner rowScanner) (board.Stage, error) {
	var value board.Stage
	err := scanner.Scan(
		&value.ID,
		&value.BoardID,
		&value.Title,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
	)
	return value, err
}

func scanStageWithBoard(scanner rowScanner) (board.Stage, string, error) {
	var value board.Stage
	var boardTitle string
	err := scanner.Scan(
		&value.ID,
		&value.BoardID,
		&value.Title,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
		&boardTitle,
	)
	return value, boardTitle, err
}

func scanShownProject(scanner rowScanner) (board.ShownProject, error) {
	var value board.ShownProject
	targets := append(projectBaseScanTargets(&value.Project), scanTagTitles(&value.Tags))
	targets = append(targets, &value.Progress.Done, &value.Progress.Total)
	err := scanner.Scan(targets...)
	return value, err
}
