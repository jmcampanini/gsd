package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
)

func TestBoardSchemaEnforcesStorageIdentityAndScopedStageNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	for _, table := range []string{"boards", "stages"} {
		var strict int
		if err := storage.database.QueryRowContext(
			ctx,
			"SELECT strict FROM pragma_table_list WHERE name = ?",
			table,
		).Scan(&strict); err != nil {
			t.Fatalf("inspect %s storage: %v", table, err)
		}
		if strict != 1 {
			t.Errorf("%s strict = %d, want 1", table, strict)
		}
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO boards (id, title, position) VALUES ('wrong', 'typed', 0)"); err == nil {
		t.Error("insert non-integer board ID error = nil, want STRICT failure")
	}

	firstBoardID := insertFixture(t, storage.database, "INSERT INTO boards (title, position) VALUES ('Delivery', 0)")
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO boards (title, position) VALUES ('delivery', 1)"); err == nil {
		t.Error("insert case-duplicate board error = nil, want NOCASE uniqueness failure")
	}
	secondBoardID := insertFixture(t, storage.database, "INSERT INTO boards (title, position) VALUES ('Personal', 1)")
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO stages (board_id, title, position) VALUES (?, 'Ready', 0)", firstBoardID); err != nil {
		t.Fatalf("insert first scoped stage: %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO stages (board_id, title, position) VALUES (?, 'ready', 1)", firstBoardID); err == nil {
		t.Error("insert case-duplicate stage on same board error = nil, want scoped NOCASE uniqueness failure")
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO stages (board_id, title, position) VALUES (?, 'ready', 0)", secondBoardID); err != nil {
		t.Fatalf("insert same stage name on another board: %v", err)
	}

	var note, createdAt, updatedAt string
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT note, created_at, updated_at FROM boards WHERE id = ?",
		firstBoardID,
	).Scan(&note, &createdAt, &updatedAt); err != nil {
		t.Fatalf("read board defaults: %v", err)
	}
	if note != "" || createdAt == "" || updatedAt == "" {
		t.Errorf("board defaults = (%q, %q, %q), want empty note and timestamps", note, createdAt, updatedAt)
	}

	var indexedColumn string
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT name FROM pragma_index_info('idx_stages_board') WHERE seqno = 0",
	).Scan(&indexedColumn); err != nil {
		t.Fatalf("inspect idx_stages_board: %v", err)
	}
	if indexedColumn != "board_id" {
		t.Errorf("idx_stages_board first column = %q, want board_id", indexedColumn)
	}
}

func TestBoardStoreCRUDUsesNoCaseNamesStoredSpellingAndOrdinalLists(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	at := "2026-01-01T00:00:00.000Z"
	delivery := addStoredBoard(t, boards, board.AddFields{Title: "Delivery", Note: "ship work"}, at)
	personal := addStoredBoard(t, boards, board.AddFields{Title: "Personal"}, at)

	found, err := boards.FindBoard(ctx, "dElIvErY")
	if err != nil {
		t.Fatalf("FindBoard(case variant) error = %v", err)
	}
	if !reflect.DeepEqual(found, delivery) {
		t.Errorf("FindBoard(case variant) = %#v, want stored row %#v", found, delivery)
	}
	if _, err := boards.AddBoard(ctx, board.AddFields{Title: "DELIVERY"}, at); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), delivery.Title) {
		t.Errorf("AddBoard(case duplicate) error = %v, want conflict naming %q", err, delivery.Title)
	}

	caseTitle := "delivery"
	note := "revised"
	edited, err := boards.EditBoard(
		ctx,
		"DELIVERY",
		board.EditFields{Title: &caseTitle, Note: &note},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("EditBoard(case only) error = %v", err)
	}
	if edited.ID != delivery.ID || edited.Title != caseTitle || edited.Note != note ||
		edited.Position != delivery.Position || edited.CreatedAt != delivery.CreatedAt ||
		edited.UpdatedAt != "2026-01-02T00:00:00.000Z" {
		t.Errorf("EditBoard(case only) = %#v, want edited complete row", edited)
	}
	conflictingTitle := "PERSONAL"
	if _, err := boards.EditBoard(ctx, "delivery", board.EditFields{Title: &conflictingTitle}, at); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), personal.Title) {
		t.Errorf("EditBoard(conflict) error = %v, want conflict naming %q", err, personal.Title)
	}

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE boards SET position = CASE id WHEN ? THEN 1 WHEN ? THEN 0 END",
		edited.ID,
		personal.ID,
	); err != nil {
		t.Fatalf("arrange board list: %v", err)
	}
	listed, err := boards.ListBoards(ctx)
	if err != nil {
		t.Fatalf("ListBoards() error = %v", err)
	}
	if got := boardIDs(listed); !reflect.DeepEqual(got, []int64{personal.ID, edited.ID}) {
		t.Errorf("ListBoards() IDs = %v, want position order", got)
	}

	ready := addStoredStage(t, boards, edited.ID, "Ready", at)
	done := addStoredStage(t, boards, edited.ID, "Done", at)
	otherReady := addStoredStage(t, boards, personal.ID, "ready", at)
	if _, err := boards.AddStage(ctx, edited.ID, "READY", at); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), ready.Title) {
		t.Errorf("AddStage(case duplicate) error = %v, want conflict naming %q", err, ready.Title)
	}
	foundStage, err := boards.FindStage(ctx, edited.ID, "rEaDy")
	if err != nil || !reflect.DeepEqual(foundStage, ready) {
		t.Errorf("FindStage(case variant) = %#v, %v; want %#v", foundStage, err, ready)
	}
	if foundOther, err := boards.FindStage(ctx, personal.ID, "READY"); err != nil || !reflect.DeepEqual(foundOther, otherReady) {
		t.Errorf("FindStage(other board) = %#v, %v; want scoped row %#v", foundOther, err, otherReady)
	}

	caseStageTitle := "ready"
	renamed, err := boards.RenameStage(ctx, edited.ID, "READY", caseStageTitle, "2026-01-03T00:00:00.000Z")
	if err != nil {
		t.Fatalf("RenameStage(case only) error = %v", err)
	}
	if renamed.ID != ready.ID || renamed.Title != caseStageTitle || renamed.UpdatedAt != "2026-01-03T00:00:00.000Z" {
		t.Errorf("RenameStage(case only) = %#v, want same row with stored new spelling", renamed)
	}
	if _, err := boards.RenameStage(ctx, edited.ID, "ready", "DONE", at); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), done.Title) {
		t.Errorf("RenameStage(conflict) error = %v, want conflict naming %q", err, done.Title)
	}

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE stages SET position = CASE id WHEN ? THEN 1 WHEN ? THEN 0 END WHERE board_id = ?",
		renamed.ID,
		done.ID,
		edited.ID,
	); err != nil {
		t.Fatalf("arrange stage list: %v", err)
	}
	stages, err := boards.ListStages(ctx, edited.ID)
	if err != nil {
		t.Fatalf("ListStages() error = %v", err)
	}
	if got := stageIDs(stages); !reflect.DeepEqual(got, []int64{done.ID, renamed.ID}) {
		t.Errorf("ListStages() IDs = %v, want board-scoped position order", got)
	}
	if empty, err := boards.ListStages(ctx, 999); err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("ListStages(unknown board) = %#v, %v; want non-nil empty list", empty, err)
	}
}

func TestBoardAndStageReordersRepairOrdinalsScopeReferencesAndTimestampOnlyMovedRows(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	at := "2026-01-01T00:00:00.000Z"
	values := []board.Board{
		addStoredBoard(t, boards, board.AddFields{Title: "one"}, at),
		addStoredBoard(t, boards, board.AddFields{Title: "two"}, at),
		addStoredBoard(t, boards, board.AddFields{Title: "three"}, at),
	}
	ids := boardIDs(values)
	setReorderFixturePositions(t, storage, "boards", ids)
	before := storedUpdatedAt(t, storage, "boards", ids)
	movedAt := "2026-02-01T00:00:00.000Z"
	moved, err := boards.ReorderBoard(
		ctx,
		values[2].ID,
		domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[1].ID},
		movedAt,
	)
	if err != nil {
		t.Fatalf("ReorderBoard() error = %v", err)
	}
	if moved.Position != 1 || moved.UpdatedAt != movedAt {
		t.Errorf("ReorderBoard() = %#v, want position 1 and moved timestamp", moved)
	}
	assertStoredOrder(t, storage, "boards", "1", nil, []int64{values[0].ID, values[2].ID, values[1].ID})
	assertMovedOnlyTimestamp(t, storage, "boards", before, moved.ID, movedAt)

	before = storedUpdatedAt(t, storage, "boards", ids)
	noopAt := "2026-02-02T00:00:00.000Z"
	if _, err := boards.ReorderBoard(ctx, values[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, noopAt); err != nil {
		t.Fatalf("ReorderBoard(no-op) error = %v", err)
	}
	assertMovedOnlyTimestamp(t, storage, "boards", before, values[0].ID, noopAt)
	if _, err := boards.ReorderBoard(ctx, values[0].ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: values[0].ID}, noopAt); errorCode(err) != apperr.InvalidArgument {
		t.Errorf("ReorderBoard(self) error = %v, want invalid_argument", err)
	}
	if _, err := boards.ReorderBoard(ctx, 999, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 998}, noopAt); errorCode(err) != apperr.NotFound {
		t.Errorf("ReorderBoard(missing subject) error = %v, want not_found", err)
	}

	firstBoard := values[0]
	otherBoard := values[1]
	stages := []board.Stage{
		addStoredStage(t, boards, firstBoard.ID, "one", at),
		addStoredStage(t, boards, firstBoard.ID, "two", at),
		addStoredStage(t, boards, firstBoard.ID, "three", at),
	}
	foreign := addStoredStage(t, boards, otherBoard.ID, "foreign", at)
	stageIDValues := stageIDs(stages)
	setReorderFixturePositions(t, storage, "stages", stageIDValues)
	before = storedUpdatedAt(t, storage, "stages", stageIDValues)
	movedStage, err := boards.ReorderStage(
		ctx,
		firstBoard.ID,
		stages[2].ID,
		domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: stages[1].ID},
		movedAt,
	)
	if err != nil {
		t.Fatalf("ReorderStage() error = %v", err)
	}
	if movedStage.Position != 1 || movedStage.UpdatedAt != movedAt {
		t.Errorf("ReorderStage() = %#v, want position 1 and moved timestamp", movedStage)
	}
	assertStoredOrder(t, storage, "stages", "board_id = ?", []any{firstBoard.ID}, []int64{stages[0].ID, stages[2].ID, stages[1].ID})
	assertStoredOrder(t, storage, "stages", "board_id = ?", []any{otherBoard.ID}, []int64{foreign.ID})
	assertMovedOnlyTimestamp(t, storage, "stages", before, movedStage.ID, movedAt)

	before = storedUpdatedAt(t, storage, "stages", stageIDValues)
	if _, err := boards.ReorderStage(ctx, firstBoard.ID, stages[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, noopAt); err != nil {
		t.Fatalf("ReorderStage(no-op) error = %v", err)
	}
	assertMovedOnlyTimestamp(t, storage, "stages", before, stages[0].ID, noopAt)
	if _, err := boards.ReorderStage(ctx, firstBoard.ID, stages[0].ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: foreign.ID}, noopAt); errorCode(err) != apperr.NotFound {
		t.Errorf("ReorderStage(cross-board reference) error = %v, want scoped not_found", err)
	}
	if _, err := boards.ReorderStage(ctx, otherBoard.ID, stages[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, noopAt); errorCode(err) != apperr.NotFound {
		t.Errorf("ReorderStage(cross-board subject) error = %v, want scoped not_found", err)
	}
}

func TestBoardAndStageDeletesReturnSnapshotsCascadeAndLeaveOrdinalGaps(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	at := "2026-01-01T00:00:00.000Z"
	kept := addStoredBoard(t, boards, board.AddFields{Title: "kept"}, at)
	doomed := addStoredBoard(t, boards, board.AddFields{Title: "doomed", Note: "snapshot"}, at)
	trailing := addStoredBoard(t, boards, board.AddFields{Title: "trailing"}, at)
	first := addStoredStage(t, boards, doomed.ID, "first", at)
	second := addStoredStage(t, boards, doomed.ID, "second", at)

	deletedStage, err := boards.DeleteStage(ctx, doomed.ID, "FIRST")
	if err != nil {
		t.Fatalf("DeleteStage() error = %v", err)
	}
	if !reflect.DeepEqual(deletedStage, first) {
		t.Errorf("DeleteStage() = %#v, want complete snapshot %#v", deletedStage, first)
	}
	remaining, err := boards.ListStages(ctx, doomed.ID)
	if err != nil || len(remaining) != 1 || remaining[0].ID != second.ID || remaining[0].Position != 1 {
		t.Errorf("stages after delete = %#v, %v; want second left at ordinal 1", remaining, err)
	}

	third := addStoredStage(t, boards, doomed.ID, "third", at)
	deletedBoard, err := boards.DeleteBoard(ctx, doomed.ID)
	if err != nil {
		t.Fatalf("DeleteBoard() error = %v", err)
	}
	if !reflect.DeepEqual(deletedBoard, doomed) {
		t.Errorf("DeleteBoard() = %#v, want complete snapshot %#v", deletedBoard, doomed)
	}
	if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM stages WHERE id IN (?, ?)", second.ID, third.ID); count != 0 {
		t.Errorf("stages after board delete = %d, want cascade to 0", count)
	}
	persisted, err := boards.ListBoards(ctx)
	if err != nil || len(persisted) != 2 || persisted[0].ID != kept.ID || persisted[0].Position != 0 ||
		persisted[1].ID != trailing.ID || persisted[1].Position != 2 {
		t.Errorf("boards after delete = %#v, %v; want surrounding boards left at ordinals 0 and 2", persisted, err)
	}
	if _, err := boards.DeleteBoard(ctx, doomed.ID); errorCode(err) != apperr.NotFound {
		t.Errorf("DeleteBoard(missing) error = %v, want not_found", err)
	}
	if _, err := boards.DeleteStage(ctx, kept.ID, "missing"); errorCode(err) != apperr.NotFound {
		t.Errorf("DeleteStage(missing) error = %v, want not_found", err)
	}
}

func TestBoardTransactionWrappersCommitReadsAndRollBackMutations(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	rollback := errors.New("roll back boards")
	err := boards.WithinTransaction(ctx, func(transaction board.Transaction) error {
		created, err := transaction.AddBoard(ctx, board.AddFields{Title: "temporary"}, "2026-01-01T00:00:00.000Z")
		if err != nil {
			return err
		}
		if _, err := transaction.AddStage(ctx, created.ID, "temporary", "2026-01-01T00:00:00.000Z"); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTransaction() error = %v, want rollback marker", err)
	}
	if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM boards"); count != 0 {
		t.Errorf("board count after rollback = %d, want 0", count)
	}

	var listed []board.Board
	if err := boards.WithinReadTransaction(ctx, func(transaction board.Transaction) error {
		var err error
		listed, err = transaction.ListBoards(ctx)
		return err
	}); err != nil {
		t.Fatalf("WithinReadTransaction() error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("read transaction list = %#v, want non-nil empty list", listed)
	}
}

func addStoredBoard(t *testing.T, boards *Boards, fields board.AddFields, timestamp string) board.Board {
	t.Helper()
	created, err := boards.AddBoard(context.Background(), fields, timestamp)
	if err != nil {
		t.Fatalf("AddBoard(%q) error = %v", fields.Title, err)
	}
	return created
}

func addStoredStage(t *testing.T, boards *Boards, boardID int64, title, timestamp string) board.Stage {
	t.Helper()
	created, err := boards.AddStage(context.Background(), boardID, title, timestamp)
	if err != nil {
		t.Fatalf("AddStage(%d, %q) error = %v", boardID, title, err)
	}
	return created
}

func boardIDs(values []board.Board) []int64 {
	ids := make([]int64, len(values))
	for index := range values {
		ids[index] = values[index].ID
	}
	return ids
}

func stageIDs(values []board.Stage) []int64 {
	ids := make([]int64, len(values))
	for index := range values {
		ids[index] = values[index].ID
	}
	return ids
}
