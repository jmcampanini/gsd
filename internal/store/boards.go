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
)

const (
	boardColumns = `id, title, note, position, created_at, updated_at`
	stageColumns = `id, board_id, title, position, created_at, updated_at`
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
	name string,
	fields board.EditFields,
	timestamp string,
) (board.Board, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Board, error) {
		return transaction.EditBoard(ctx, name, fields, timestamp)
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

func (s *Boards) RenameStage(
	ctx context.Context,
	boardID int64,
	oldName, newName, timestamp string,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.RenameStage(ctx, boardID, oldName, newName, timestamp)
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
	boardID int64,
	name string,
) (board.Stage, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction board.Transaction) (board.Stage, error) {
		return transaction.DeleteStage(ctx, boardID, name)
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
	name string,
	fields board.EditFields,
	timestamp string,
) (board.Board, error) {
	if fields.Title == nil && fields.Note == nil {
		return board.Board{}, errors.New("board edit requires at least one field")
	}
	current, err := s.FindBoard(ctx, name)
	if err != nil {
		return board.Board{}, err
	}
	if fields.Title != nil {
		existing, findErr := s.FindBoard(ctx, *fields.Title)
		switch code, coded := apperr.CodeOf(findErr); {
		case findErr == nil && existing.ID != current.ID:
			return board.Board{}, apperr.New(
				apperr.Conflict,
				fmt.Sprintf("board already exists: %s", existing.Title),
				nil,
			)
		case findErr == nil:
		case coded && code == apperr.NotFound:
		default:
			return board.Board{}, fmt.Errorf("check board edit title: %w", findErr)
		}
	}

	assignments := make([]string, 0, 3)
	arguments := make([]any, 0, 4)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}
	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, current.ID)

	edited, err := scanBoard(s.executor.QueryRowContext(
		ctx,
		"UPDATE boards SET "+strings.Join(assignments, ", ")+" WHERE id = ? RETURNING "+boardColumns,
		arguments...,
	))
	if err != nil {
		return board.Board{}, fmt.Errorf("edit board: %w", err)
	}
	return edited, nil
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
	found, err := scanStage(s.executor.QueryRowContext(
		ctx,
		"SELECT "+stageColumns+" FROM stages WHERE board_id = ? AND title = ? COLLATE NOCASE",
		boardID,
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return board.Stage{}, apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no stage %s on board %d", name, boardID),
			err,
		)
	}
	if err != nil {
		return board.Stage{}, fmt.Errorf("find stage: %w", err)
	}
	return found, nil
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

func (s *boardsCore) RenameStage(
	ctx context.Context,
	boardID int64,
	oldName, newName, timestamp string,
) (board.Stage, error) {
	current, err := s.FindStage(ctx, boardID, oldName)
	if err != nil {
		return board.Stage{}, err
	}
	existing, findErr := s.FindStage(ctx, boardID, newName)
	switch code, coded := apperr.CodeOf(findErr); {
	case findErr == nil && existing.ID != current.ID:
		return board.Stage{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("stage already exists: %s", existing.Title),
			nil,
		)
	case findErr == nil:
	case coded && code == apperr.NotFound:
	default:
		return board.Stage{}, fmt.Errorf("check stage rename title: %w", findErr)
	}

	renamed, err := scanStage(s.executor.QueryRowContext(ctx, `
UPDATE stages
SET title = ?, updated_at = ?
WHERE board_id = ? AND id = ?
RETURNING `+stageColumns, newName, timestamp, boardID, current.ID))
	if err != nil {
		return board.Stage{}, fmt.Errorf("rename stage: %w", err)
	}
	return renamed, nil
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
	boardID int64,
	name string,
) (board.Stage, error) {
	deleted, err := s.FindStage(ctx, boardID, name)
	if err != nil {
		return board.Stage{}, err
	}
	if err := deleteRows(
		ctx,
		s.executor,
		1,
		"DELETE FROM stages WHERE board_id = ? AND id = ?",
		boardID,
		deleted.ID,
	); err != nil {
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
