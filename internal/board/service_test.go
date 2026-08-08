package board

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
)

type fakeStore struct {
	calls                 []string
	addBoardResult        Board
	addBoardError         error
	addBoardFields        AddFields
	addBoardTimestamp     string
	findBoards            map[string]Board
	findBoardError        error
	listBoardsResult      []Board
	listBoardsError       error
	editBoardResult       Board
	editBoardTitle        string
	editBoardFields       EditFields
	editBoardTimestamp    string
	reorderBoardResult    Board
	reorderBoardID        int64
	reorderBoardPlacement domain.Placement
	reorderBoardTimestamp string
	deleteBoardResult     Board
	deleteBoardID         int64
	addStageResults       []Stage
	addStageErrorAt       int
	addStageCalls         int
	addStageBoardIDs      []int64
	addStageTitles        []string
	addStageTimestamps    []string
	findStages            map[string]Stage
	findStageError        error
	findStageBoardIDs     []int64
	listStages            map[int64][]Stage
	listStagesError       error
	renameStageResult     Stage
	renameStageBoardID    int64
	renameStageOldTitle   string
	renameStageTitle      string
	renameStageTimestamp  string
	reorderStageResult    Stage
	reorderStageBoardID   int64
	reorderStageID        int64
	reorderStagePlacement domain.Placement
	reorderStageTimestamp string
	deleteStageResult     Stage
	deleteStageBoardID    int64
	deleteStageTitle      string
	transactionStore      Transaction
	transactionError      error
	transactionCalls      int
	readTransactionStore  Transaction
	readTransactionError  error
	readTransactionCalls  int
}

func (f *fakeStore) AddBoard(_ context.Context, fields AddFields, timestamp string) (Board, error) {
	f.calls = append(f.calls, "add board")
	f.addBoardFields = fields
	f.addBoardTimestamp = timestamp
	return f.addBoardResult, f.addBoardError
}

func (f *fakeStore) FindBoard(_ context.Context, title string) (Board, error) {
	f.calls = append(f.calls, "find board "+title)
	if f.findBoardError != nil {
		return Board{}, f.findBoardError
	}
	return f.findBoards[title], nil
}

func (f *fakeStore) ListBoards(context.Context) ([]Board, error) {
	f.calls = append(f.calls, "list boards")
	return f.listBoardsResult, f.listBoardsError
}

func (f *fakeStore) EditBoard(_ context.Context, title string, fields EditFields, timestamp string) (Board, error) {
	f.calls = append(f.calls, "edit board")
	f.editBoardTitle = title
	f.editBoardFields = fields
	f.editBoardTimestamp = timestamp
	return f.editBoardResult, nil
}

func (f *fakeStore) ReorderBoard(_ context.Context, id int64, placement domain.Placement, timestamp string) (Board, error) {
	f.calls = append(f.calls, "reorder board")
	f.reorderBoardID = id
	f.reorderBoardPlacement = placement
	f.reorderBoardTimestamp = timestamp
	return f.reorderBoardResult, nil
}

func (f *fakeStore) DeleteBoard(_ context.Context, id int64) (Board, error) {
	f.calls = append(f.calls, "delete board")
	f.deleteBoardID = id
	return f.deleteBoardResult, nil
}

func (f *fakeStore) AddStage(_ context.Context, boardID int64, title, timestamp string) (Stage, error) {
	f.calls = append(f.calls, "add stage "+title)
	f.addStageCalls++
	f.addStageBoardIDs = append(f.addStageBoardIDs, boardID)
	f.addStageTitles = append(f.addStageTitles, title)
	f.addStageTimestamps = append(f.addStageTimestamps, timestamp)
	if f.addStageErrorAt == f.addStageCalls {
		return Stage{}, errors.New("add stage failed")
	}
	if f.addStageCalls <= len(f.addStageResults) {
		return f.addStageResults[f.addStageCalls-1], nil
	}
	return Stage{}, nil
}

func (f *fakeStore) FindStage(_ context.Context, boardID int64, title string) (Stage, error) {
	f.calls = append(f.calls, "find stage "+title)
	f.findStageBoardIDs = append(f.findStageBoardIDs, boardID)
	if f.findStageError != nil {
		return Stage{}, f.findStageError
	}
	return f.findStages[title], nil
}

func (f *fakeStore) ListStages(_ context.Context, boardID int64) ([]Stage, error) {
	f.calls = append(f.calls, "list stages")
	return f.listStages[boardID], f.listStagesError
}

func (f *fakeStore) RenameStage(_ context.Context, boardID int64, oldTitle, title, timestamp string) (Stage, error) {
	f.calls = append(f.calls, "rename stage")
	f.renameStageBoardID = boardID
	f.renameStageOldTitle = oldTitle
	f.renameStageTitle = title
	f.renameStageTimestamp = timestamp
	return f.renameStageResult, nil
}

func (f *fakeStore) ReorderStage(_ context.Context, boardID, id int64, placement domain.Placement, timestamp string) (Stage, error) {
	f.calls = append(f.calls, "reorder stage")
	f.reorderStageBoardID = boardID
	f.reorderStageID = id
	f.reorderStagePlacement = placement
	f.reorderStageTimestamp = timestamp
	return f.reorderStageResult, nil
}

func (f *fakeStore) DeleteStage(_ context.Context, boardID int64, title string) (Stage, error) {
	f.calls = append(f.calls, "delete stage")
	f.deleteStageBoardID = boardID
	f.deleteStageTitle = title
	return f.deleteStageResult, nil
}

func (f *fakeStore) WithinTransaction(ctx context.Context, operation func(Transaction) error) error {
	f.transactionCalls++
	if f.transactionError != nil {
		return f.transactionError
	}
	transaction := f.transactionStore
	if transaction == nil {
		transaction = f
	}
	return operation(transaction)
}

func (f *fakeStore) WithinReadTransaction(ctx context.Context, operation func(Transaction) error) error {
	f.readTransactionCalls++
	if f.readTransactionError != nil {
		return f.readTransactionError
	}
	transaction := f.readTransactionStore
	if transaction == nil {
		transaction = f
	}
	return operation(transaction)
}

func TestInvalidInputsAreRejectedBeforeStoreOrClock(t *testing.T) {
	t.Parallel()

	valid := "valid"
	blank := " \t"
	invalidUTF8 := string([]byte{0xff})
	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{name: "add board title", apply: func(s *Service) error {
			_, err := s.Add(context.Background(), AddFields{Title: blank, Stages: []string{valid}})
			return err
		}},
		{name: "add board note", apply: func(s *Service) error {
			_, err := s.Add(context.Background(), AddFields{Title: valid, Note: invalidUTF8, Stages: []string{valid}})
			return err
		}},
		{name: "add without stages", apply: func(s *Service) error { _, err := s.Add(context.Background(), AddFields{Title: valid}); return err }},
		{name: "add stage title", apply: func(s *Service) error {
			_, err := s.Add(context.Background(), AddFields{Title: valid, Stages: []string{blank}})
			return err
		}},
		{name: "show board title", apply: func(s *Service) error { _, err := s.Show(context.Background(), blank); return err }},
		{name: "edit without fields", apply: func(s *Service) error { _, err := s.Edit(context.Background(), valid, EditFields{}); return err }},
		{name: "edit title", apply: func(s *Service) error {
			_, err := s.Edit(context.Background(), valid, EditFields{Title: &blank})
			return err
		}},
		{name: "edit note", apply: func(s *Service) error {
			_, err := s.Edit(context.Background(), valid, EditFields{Note: &invalidUTF8})
			return err
		}},
		{name: "reorder reference", apply: func(s *Service) error {
			_, err := s.Reorder(context.Background(), valid, Placement{Anchor: domain.PlacementAfter, Reference: blank})
			return err
		}},
		{name: "delete board title", apply: func(s *Service) error { _, err := s.Delete(context.Background(), blank); return err }},
		{name: "add stage board", apply: func(s *Service) error {
			_, err := s.AddStage(context.Background(), blank, valid, Placement{})
			return err
		}},
		{name: "add stage", apply: func(s *Service) error {
			_, err := s.AddStage(context.Background(), valid, blank, Placement{})
			return err
		}},
		{name: "rename old stage", apply: func(s *Service) error { _, err := s.RenameStage(context.Background(), valid, blank, valid); return err }},
		{name: "rename new stage", apply: func(s *Service) error { _, err := s.RenameStage(context.Background(), valid, valid, blank); return err }},
		{name: "stage reorder anchor", apply: func(s *Service) error {
			_, err := s.ReorderStage(context.Background(), valid, valid, Placement{})
			return err
		}},
		{name: "stage delete board", apply: func(s *Service) error { _, err := s.DeleteStage(context.Background(), blank, valid); return err }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeStore{}
			service := NewService(store)
			clockCalls := 0
			service.now = func() time.Time { clockCalls++; return time.Time{} }
			err := test.apply(service)
			if code, _ := apperr.CodeOf(err); code != apperr.InvalidArgument {
				t.Fatalf("operation error = %v, want invalid_argument", err)
			}
			if store.transactionCalls != 0 || store.readTransactionCalls != 0 || len(store.calls) != 0 || clockCalls != 0 {
				t.Errorf("boundary/direct/clock calls = %d/%d/%v/%d, want none", store.transactionCalls, store.readTransactionCalls, store.calls, clockCalls)
			}
		})
	}
}

func TestAddUsesOneTransactionTimestampAndInputStageOrder(t *testing.T) {
	t.Parallel()

	fields := AddFields{Title: "  Delivery  ", Note: "notes\n", Stages: []string{"Ideas", "Doing"}}
	transaction := &fakeStore{
		addBoardResult:  Board{ID: 4, Title: fields.Title, Note: fields.Note},
		addStageResults: []Stage{{ID: 10, BoardID: 4, Title: "Ideas"}, {ID: 11, BoardID: 4, Title: "Doing"}},
	}
	store := &fakeStore{transactionStore: transaction}
	service := NewService(store)
	clockCalls := 0
	service.now = func() time.Time {
		clockCalls++
		return time.Date(2026, time.August, 8, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	got, err := service.Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.transactionCalls != 1 || len(store.calls) != 0 {
		t.Errorf("outer store = %#v, want transaction boundary only", store)
	}
	if !reflect.DeepEqual(transaction.calls, []string{"add board", "add stage Ideas", "add stage Doing"}) {
		t.Errorf("transaction calls = %v, want board then stages in input order", transaction.calls)
	}
	if !reflect.DeepEqual(transaction.addBoardFields, fields) || !reflect.DeepEqual(transaction.addStageBoardIDs, []int64{4, 4}) || !reflect.DeepEqual(transaction.addStageTitles, fields.Stages) {
		t.Errorf("add delegation = %#v, want unchanged fields on board 4", transaction)
	}
	wantTimestamp := "2026-08-08T16:34:56.987Z"
	if clockCalls != 1 || transaction.addBoardTimestamp != wantTimestamp || !reflect.DeepEqual(transaction.addStageTimestamps, []string{wantTimestamp, wantTimestamp}) {
		t.Errorf("clock/timestamps = %d/%q/%v, want one clock and one timestamp", clockCalls, transaction.addBoardTimestamp, transaction.addStageTimestamps)
	}
	if got.Stages == nil || !reflect.DeepEqual(got, Addition{Board: transaction.addBoardResult, Stages: transaction.addStageResults}) {
		t.Errorf("Add() = %#v, want stored board and stages", got)
	}
}

func TestAddReturnsTransactionFailureWithoutPartialResult(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{addBoardResult: Board{ID: 4}, addStageErrorAt: 2}
	store := &fakeStore{transactionStore: transaction}
	got, err := NewService(store).Add(context.Background(), AddFields{Title: "Board", Stages: []string{"One", "Two"}})
	if err == nil {
		t.Fatal("Add() error = nil, want stage failure")
	}
	if !reflect.DeepEqual(got, Addition{}) || store.transactionCalls != 1 || !reflect.DeepEqual(transaction.calls, []string{"add board", "add stage One", "add stage Two"}) {
		t.Errorf("Add() result/delegation = %#v/%v, want zero result and one failed transaction", got, transaction.calls)
	}
}

func TestReordersResolveNamesToIDsInsideWriteTransaction(t *testing.T) {
	t.Parallel()

	boardTransaction := &fakeStore{
		findBoards: map[string]Board{
			"WORK": {ID: 7, Title: "Work"},
			"HOME": {ID: 9, Title: "Home"},
		},
		reorderBoardResult: Board{ID: 7, Title: "Work", Position: 2},
	}
	boardStore := &fakeStore{transactionStore: boardTransaction}
	gotBoard, err := NewService(boardStore).Reorder(context.Background(), "WORK", Placement{Anchor: domain.PlacementAfter, Reference: "HOME"})
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if gotBoard != boardTransaction.reorderBoardResult || boardStore.transactionCalls != 1 || !reflect.DeepEqual(boardTransaction.calls, []string{"find board WORK", "find board HOME", "reorder board"}) || boardTransaction.reorderBoardID != 7 || boardTransaction.reorderBoardPlacement != (domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 9}) {
		t.Errorf("board reorder result/delegation = %#v/%#v", gotBoard, boardTransaction)
	}

	stageTransaction := &fakeStore{
		findBoards: map[string]Board{"WORK": {ID: 7, Title: "Work"}},
		findStages: map[string]Stage{
			"DOING": {ID: 12, BoardID: 7, Title: "Doing"},
			"DONE":  {ID: 13, BoardID: 7, Title: "Done"},
		},
		reorderStageResult: Stage{ID: 12, BoardID: 7, Title: "Doing", Position: 2},
	}
	stageStore := &fakeStore{transactionStore: stageTransaction}
	gotStage, err := NewService(stageStore).ReorderStage(context.Background(), "WORK", "DOING", Placement{Anchor: domain.PlacementBefore, Reference: "DONE"})
	if err != nil {
		t.Fatalf("ReorderStage() error = %v", err)
	}
	wantStage := StageResult{Board: stageTransaction.findBoards["WORK"], Stage: stageTransaction.reorderStageResult}
	if !reflect.DeepEqual(gotStage, wantStage) || stageStore.transactionCalls != 1 || !reflect.DeepEqual(stageTransaction.calls, []string{"find board WORK", "find stage DOING", "find stage DONE", "reorder stage"}) || stageTransaction.reorderStageBoardID != 7 || stageTransaction.reorderStageID != 12 || stageTransaction.reorderStagePlacement != (domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 13}) || !reflect.DeepEqual(stageTransaction.findStageBoardIDs, []int64{7, 7}) {
		t.Errorf("stage reorder result/delegation = %#v/%#v", gotStage, stageTransaction)
	}
}

func TestRelativeReorderRejectsResolvedSelfReference(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{findBoards: map[string]Board{
		"work": {ID: 7, Title: "Work"},
		"WORK": {ID: 7, Title: "Work"},
	}}
	store := &fakeStore{transactionStore: transaction}
	_, err := NewService(store).Reorder(context.Background(), "work", Placement{Anchor: domain.PlacementAfter, Reference: "WORK"})
	if code, _ := apperr.CodeOf(err); code != apperr.InvalidArgument {
		t.Fatalf("Reorder() error = %v, want invalid_argument", err)
	}
	if !reflect.DeepEqual(transaction.calls, []string{"find board work", "find board WORK"}) {
		t.Errorf("transaction calls = %v, want both resolutions and no mutation", transaction.calls)
	}
}

func TestAddStageDefaultsToAppendWithoutSecondMutation(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{
		findBoards:      map[string]Board{"work": {ID: 7, Title: "Work"}},
		addStageResults: []Stage{{ID: 15, BoardID: 7, Title: "Review", Position: 3}},
	}
	store := &fakeStore{transactionStore: transaction}
	service := NewService(store)
	service.now = func() time.Time { return time.Date(2026, time.August, 8, 1, 2, 3, 0, time.UTC) }
	got, err := service.AddStage(context.Background(), "work", "Review", Placement{})
	if err != nil {
		t.Fatalf("AddStage() error = %v", err)
	}
	want := StageResult{Board: transaction.findBoards["work"], Stage: transaction.addStageResults[0]}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(transaction.calls, []string{"find board work", "add stage Review"}) || transaction.addStageBoardIDs[0] != 7 || transaction.addStageTimestamps[0] != "2026-08-08T01:02:03.000Z" {
		t.Errorf("AddStage() result/delegation = %#v/%#v, want append only", got, transaction)
	}
}

func TestAddStageRelativePlacementAppendsThenReordersInTheSameTransaction(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{
		findBoards:      map[string]Board{"work": {ID: 7, Title: "Work"}},
		findStages:      map[string]Stage{"Doing": {ID: 12, BoardID: 7, Title: "Doing"}},
		addStageResults: []Stage{{ID: 15, BoardID: 7, Title: "Review", Position: 4}},
		reorderStageResult: Stage{
			ID: 15, BoardID: 7, Title: "Review", Position: 3,
		},
	}
	store := &fakeStore{transactionStore: transaction}
	service := NewService(store)
	service.now = func() time.Time {
		return time.Date(2026, time.August, 8, 1, 2, 3, 456000000, time.UTC)
	}

	got, err := service.AddStage(
		context.Background(),
		"work",
		"Review",
		Placement{Anchor: domain.PlacementBefore, Reference: "Doing"},
	)
	if err != nil {
		t.Fatalf("AddStage() error = %v", err)
	}
	want := StageResult{Board: transaction.findBoards["work"], Stage: transaction.reorderStageResult}
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(transaction.calls, []string{"find board work", "add stage Review", "find stage Doing", "reorder stage"}) {
		t.Errorf("AddStage() result/calls = %#v/%v, want append, resolve, reorder", got, transaction.calls)
	}
	wantPlacement := domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 12}
	wantTimestamp := "2026-08-08T01:02:03.456Z"
	if transaction.reorderStageBoardID != 7 || transaction.reorderStageID != 15 || transaction.reorderStagePlacement != wantPlacement || transaction.addStageTimestamps[0] != wantTimestamp || transaction.reorderStageTimestamp != wantTimestamp {
		t.Errorf("AddStage() reorder delegation = %#v, want board/stage IDs, numeric placement, and one timestamp", transaction)
	}
}

func TestListAndShowAssembleNonnilSlicesInReadTransactions(t *testing.T) {
	t.Parallel()

	listTransaction := &fakeStore{
		listBoardsResult: []Board{{ID: 7, Title: "Work"}},
		listStages:       map[int64][]Stage{},
	}
	listStore := &fakeStore{readTransactionStore: listTransaction}
	listed, err := NewService(listStore).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listStore.readTransactionCalls != 1 || len(listed) != 1 || listed[0].Stages == nil || len(listed[0].Stages) != 0 || !reflect.DeepEqual(listTransaction.calls, []string{"list boards", "list stages"}) {
		t.Errorf("List() result/read = %#v/%#v, want coherent board with empty stages", listed, listTransaction)
	}

	stage := Stage{ID: 10, BoardID: 7, Title: "Ideas"}
	showTransaction := &fakeStore{
		findBoards: map[string]Board{"work": {ID: 7, Title: "Work"}},
		listStages: map[int64][]Stage{7: {stage}},
	}
	showStore := &fakeStore{readTransactionStore: showTransaction}
	shown, err := NewService(showStore).Show(context.Background(), "work")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if showStore.readTransactionCalls != 1 || shown.Stages == nil || len(shown.Stages) != 1 || shown.Stages[0].Projects == nil || len(shown.Stages[0].Projects) != 0 || shown.Stages[0].Stage != stage {
		t.Errorf("Show() = %#v, want every stage with non-nil empty projects", shown)
	}

	emptyStore := &fakeStore{readTransactionStore: &fakeStore{findBoards: map[string]Board{"empty": {ID: 8}}, listStages: map[int64][]Stage{}}}
	empty, err := NewService(emptyStore).Show(context.Background(), "empty")
	if err != nil || empty.Stages == nil || len(empty.Stages) != 0 {
		t.Errorf("empty Show() = %#v, %v, want non-nil empty stages", empty, err)
	}

	nilStore := &fakeStore{readTransactionStore: &fakeStore{listStages: map[int64][]Stage{}}}
	none, err := NewService(nilStore).List(context.Background())
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("empty List() = %#v, %v, want non-nil empty list", none, err)
	}
}

func TestDeleteSnapshotsOrderedStagesBeforeBoardDeletion(t *testing.T) {
	t.Parallel()

	stages := []Stage{{ID: 10, BoardID: 7, Title: "Ideas", Position: 1}, {ID: 11, BoardID: 7, Title: "Doing", Position: 2}}
	transaction := &fakeStore{
		findBoards:        map[string]Board{"work": {ID: 7, Title: "Work"}},
		listStages:        map[int64][]Stage{7: stages},
		deleteBoardResult: Board{ID: 7, Title: "Work"},
	}
	store := &fakeStore{transactionStore: transaction}
	got, err := NewService(store).Delete(context.Background(), "work")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	want := Deletion{Board: transaction.deleteBoardResult, Stages: stages}
	if !reflect.DeepEqual(got, want) || store.transactionCalls != 1 || transaction.deleteBoardID != 7 || !reflect.DeepEqual(transaction.calls, []string{"find board work", "list stages", "delete board"}) {
		t.Errorf("Delete() result/delegation = %#v/%#v, want ordered snapshot before deletion", got, transaction)
	}
}

func TestRenameStageReturnsStoredBoardAndPreviousSpelling(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{
		findBoards:        map[string]Board{"WORK": {ID: 7, Title: "Work"}},
		findStages:        map[string]Stage{"doing": {ID: 12, BoardID: 7, Title: "Doing"}},
		renameStageResult: Stage{ID: 12, BoardID: 7, Title: "In Progress"},
	}
	store := &fakeStore{transactionStore: transaction}
	got, err := NewService(store).RenameStage(context.Background(), "WORK", "doing", "In Progress")
	if err != nil {
		t.Fatalf("RenameStage() error = %v", err)
	}
	want := StageRenameResult{Board: transaction.findBoards["WORK"], Stage: transaction.renameStageResult, PreviousTitle: "Doing"}
	if !reflect.DeepEqual(got, want) || transaction.renameStageBoardID != 7 || transaction.renameStageOldTitle != "Doing" || transaction.renameStageTitle != "In Progress" {
		t.Errorf("RenameStage() = %#v/%#v, want stored context and old spelling", got, transaction)
	}
}
