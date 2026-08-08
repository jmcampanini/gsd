package board

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeStore struct {
	calls                     []string
	addBoardResult            Board
	addBoardError             error
	addBoardFields            AddFields
	addBoardTimestamp         string
	findBoards                map[string]Board
	findBoardError            error
	listBoardsResult          []Board
	listBoardsError           error
	editBoardResult           Board
	editBoardID               int64
	editBoardFields           EditFields
	editBoardTimestamp        string
	reorderBoardResult        Board
	reorderBoardID            int64
	reorderBoardPlacement     domain.Placement
	reorderBoardTimestamp     string
	deleteBoardResult         Board
	deleteBoardID             int64
	addStageResults           []Stage
	addStageErrorAt           int
	addStageCalls             int
	addStageBoardIDs          []int64
	addStageTitles            []string
	addStageTimestamps        []string
	findStages                map[string]Stage
	findStageError            error
	findStageBoardIDs         []int64
	listStages                map[int64][]Stage
	listStagesError           error
	listShownProjectsResult   []ShownProject
	listShownProjectsError    error
	listShownProjectsBoardIDs []int64
	boardOccupied             bool
	boardOccupiedError        error
	boardOccupiedIDs          []int64
	stageOccupied             bool
	stageOccupiedError        error
	stageOccupiedIDs          []int64
	clearDefersResult         []task.Task
	clearDefersError          error
	clearDefersCalls          int
	clearDefersStageID        int64
	clearDefersTimestamp      string
	renameStageResult         Stage
	renameStageBoardID        int64
	renameStageID             int64
	renameStageTitle          string
	renameStageTimestamp      string
	reorderStageResult        Stage
	reorderStageBoardID       int64
	reorderStageID            int64
	reorderStagePlacement     domain.Placement
	reorderStageTimestamp     string
	deleteStageResult         Stage
	deleteStageBoardID        int64
	deleteStageID             int64
	transactionStore          Transaction
	transactionError          error
	transactionCalls          int
	readTransactionStore      Transaction
	readTransactionError      error
	readTransactionCalls      int
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
	found, exists := f.findBoards[title]
	if !exists {
		return Board{}, apperr.New(apperr.NotFound, "no board "+title, nil)
	}
	return found, nil
}

func (f *fakeStore) ListBoards(context.Context) ([]Board, error) {
	f.calls = append(f.calls, "list boards")
	return f.listBoardsResult, f.listBoardsError
}

func (f *fakeStore) EditBoard(_ context.Context, id int64, fields EditFields, timestamp string) (Board, error) {
	f.calls = append(f.calls, "edit board")
	f.editBoardID = id
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
	found, exists := f.findStages[title]
	if !exists {
		return Stage{}, apperr.New(apperr.NotFound, "no stage "+title, nil)
	}
	return found, nil
}

func (f *fakeStore) ListStages(_ context.Context, boardID int64) ([]Stage, error) {
	f.calls = append(f.calls, "list stages")
	return f.listStages[boardID], f.listStagesError
}

func (f *fakeStore) ListShownProjects(_ context.Context, boardID int64) ([]ShownProject, error) {
	f.calls = append(f.calls, "list shown projects")
	f.listShownProjectsBoardIDs = append(f.listShownProjectsBoardIDs, boardID)
	return f.listShownProjectsResult, f.listShownProjectsError
}

func (f *fakeStore) BoardOccupied(_ context.Context, boardID int64) (bool, error) {
	f.calls = append(f.calls, "board occupied")
	f.boardOccupiedIDs = append(f.boardOccupiedIDs, boardID)
	return f.boardOccupied, f.boardOccupiedError
}

func (f *fakeStore) StageOccupied(_ context.Context, stageID int64) (bool, error) {
	f.calls = append(f.calls, "stage occupied")
	f.stageOccupiedIDs = append(f.stageOccupiedIDs, stageID)
	return f.stageOccupied, f.stageOccupiedError
}

func (f *fakeStore) ClearTaskStageDefers(
	_ context.Context,
	stageID int64,
	timestamp string,
) ([]task.Task, error) {
	f.calls = append(f.calls, "clear defers")
	f.clearDefersCalls++
	f.clearDefersStageID = stageID
	f.clearDefersTimestamp = timestamp
	return f.clearDefersResult, f.clearDefersError
}

func (f *fakeStore) RenameStage(_ context.Context, boardID, id int64, title, timestamp string) (Stage, error) {
	f.calls = append(f.calls, "rename stage")
	f.renameStageBoardID = boardID
	f.renameStageID = id
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

func (f *fakeStore) DeleteStage(_ context.Context, boardID, id int64) (Stage, error) {
	f.calls = append(f.calls, "delete stage")
	f.deleteStageBoardID = boardID
	f.deleteStageID = id
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

func callPrecedes(calls []string, before, after string) bool {
	beforeIndex := -1
	afterIndex := -1
	for index, call := range calls {
		switch call {
		case before:
			beforeIndex = index
		case after:
			afterIndex = index
		}
	}
	return beforeIndex >= 0 && afterIndex >= 0 && beforeIndex < afterIndex
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
	if !reflect.DeepEqual(got, Addition{}) || store.transactionCalls != 1 || transaction.addStageCalls != 2 ||
		!reflect.DeepEqual(transaction.addStageTitles, []string{"One", "Two"}) {
		t.Errorf("Add() result/delegation = %#v/%#v, want zero result after second stage fails", got, transaction)
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
	if gotBoard != boardTransaction.reorderBoardResult || boardStore.transactionCalls != 1 || boardTransaction.reorderBoardID != 7 || boardTransaction.reorderBoardPlacement != (domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 9}) {
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
	if !reflect.DeepEqual(gotStage, wantStage) || stageStore.transactionCalls != 1 || stageTransaction.reorderStageBoardID != 7 || stageTransaction.reorderStageID != 12 || stageTransaction.reorderStagePlacement != (domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 13}) || !reflect.DeepEqual(stageTransaction.findStageBoardIDs, []int64{7, 7}) {
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
	if transaction.reorderBoardID != 0 {
		t.Errorf("ReorderBoard() ID = %d, want no mutation", transaction.reorderBoardID)
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
	if !reflect.DeepEqual(got, want) || transaction.addStageCalls != 1 || transaction.reorderStageID != 0 || transaction.addStageBoardIDs[0] != 7 || transaction.addStageTimestamps[0] != "2026-08-08T01:02:03.000Z" {
		t.Errorf("AddStage() result/delegation = %#v/%#v, want append only", got, transaction)
	}
}

func TestAddStageRelativePlacementResolvesThenAppendsAndReordersInSameTransaction(t *testing.T) {
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
	if !reflect.DeepEqual(got, want) || transaction.addStageCalls != 1 {
		t.Errorf("AddStage() result/add calls = %#v/%d, want resolved append and reorder", got, transaction.addStageCalls)
	}
	wantPlacement := domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 12}
	wantTimestamp := "2026-08-08T01:02:03.456Z"
	if transaction.reorderStageBoardID != 7 || transaction.reorderStageID != 15 || transaction.reorderStagePlacement != wantPlacement || transaction.addStageTimestamps[0] != wantTimestamp || transaction.reorderStageTimestamp != wantTimestamp {
		t.Errorf("AddStage() reorder delegation = %#v, want board/stage IDs, numeric placement, and one timestamp", transaction)
	}
}

func TestAddStageResolvesUnknownRelativePlacementBeforeInsert(t *testing.T) {
	t.Parallel()

	transaction := &fakeStore{
		findBoards:     map[string]Board{"work": {ID: 7, Title: "Work"}},
		findStageError: apperr.New(apperr.NotFound, "no stage Review", nil),
	}
	store := &fakeStore{transactionStore: transaction}
	_, err := NewService(store).AddStage(
		context.Background(),
		"work",
		"Review",
		Placement{Anchor: domain.PlacementAfter, Reference: "Review"},
	)
	if code, _ := apperr.CodeOf(err); code != apperr.NotFound ||
		!strings.Contains(err.Error(), "Review") || !strings.Contains(err.Error(), "Work") {
		t.Fatalf("AddStage() error = %v, want stored-board not_found", err)
	}
	if transaction.addStageCalls != 0 || transaction.reorderStageID != 0 {
		t.Errorf("AddStage()/ReorderStage() calls = %d/%d, want no mutation", transaction.addStageCalls, transaction.reorderStageID)
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
	if listStore.readTransactionCalls != 1 || len(listed) != 1 || listed[0].Stages == nil || len(listed[0].Stages) != 0 {
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

func TestShowGroupsPopulatedProjectsByStageWithNonnilArrays(t *testing.T) {
	t.Parallel()

	ideas := Stage{ID: 10, BoardID: 7, Title: "Ideas", Position: 0}
	doing := Stage{ID: 11, BoardID: 7, Title: "Doing", Position: 1}
	review := Stage{ID: 12, BoardID: 7, Title: "Review", Position: 2}
	ideasID := ideas.ID
	doingID := doing.ID
	firstIdea := ShownProject{
		Project:  project.Project{ID: 21, StageID: &ideasID, Title: "First idea"},
		Progress: ProjectProgress{Done: 1, Total: 3},
	}
	secondIdea := ShownProject{
		Project:  project.Project{ID: 22, StageID: &ideasID, Title: "Second idea"},
		Progress: ProjectProgress{Done: 0, Total: 2},
	}
	active := ShownProject{
		Project:  project.Project{ID: 23, StageID: &doingID, Title: "Active"},
		Progress: ProjectProgress{Done: 4, Total: 5},
	}
	transaction := &fakeStore{
		findBoards:              map[string]Board{"work": {ID: 7, Title: "Work"}},
		listStages:              map[int64][]Stage{7: {ideas, doing, review}},
		listShownProjectsResult: []ShownProject{firstIdea, secondIdea, active},
	}
	store := &fakeStore{readTransactionStore: transaction}

	shown, err := NewService(store).Show(context.Background(), "work")
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if store.readTransactionCalls != 1 ||
		!reflect.DeepEqual(transaction.listShownProjectsBoardIDs, []int64{7}) {
		t.Errorf("Show() read delegation = %#v/%#v, want one board-scoped read transaction", store, transaction)
	}
	want := Show{
		Board: transaction.findBoards["work"],
		Stages: []ShownStage{
			{Stage: ideas, Projects: []ShownProject{firstIdea, secondIdea}},
			{Stage: doing, Projects: []ShownProject{active}},
			{Stage: review, Projects: []ShownProject{}},
		},
	}
	if !reflect.DeepEqual(shown, want) {
		t.Errorf("Show() = %#v, want grouped result %#v", shown, want)
	}
	for _, stage := range shown.Stages {
		if stage.Projects == nil {
			t.Errorf("stage %q projects = nil, want non-nil array", stage.Title)
		}
	}
}

func TestOccupiedBoardAndStageDeletesConflictBeforeDelete(t *testing.T) {
	t.Parallel()

	t.Run("board", func(t *testing.T) {
		t.Parallel()

		transaction := &fakeStore{
			findBoards:        map[string]Board{"work": {ID: 7, Title: "Work"}},
			boardOccupied:     true,
			deleteBoardResult: Board{ID: 7, Title: "Work"},
		}
		store := &fakeStore{transactionStore: transaction}
		got, err := NewService(store).Delete(context.Background(), "work")
		if code, _ := apperr.CodeOf(err); code != apperr.Conflict {
			t.Fatalf("Delete() error = %v, want conflict", err)
		}
		if !reflect.DeepEqual(got, Deletion{}) || store.transactionCalls != 1 ||
			!reflect.DeepEqual(transaction.boardOccupiedIDs, []int64{7}) || transaction.deleteBoardID != 0 {
			t.Errorf("Delete() result/delegation = %#v/%#v, want conflict before delete", got, transaction)
		}
	})

	t.Run("stage", func(t *testing.T) {
		t.Parallel()

		transaction := &fakeStore{
			findBoards:        map[string]Board{"work": {ID: 7, Title: "Work"}},
			findStages:        map[string]Stage{"doing": {ID: 11, BoardID: 7, Title: "Doing"}},
			stageOccupied:     true,
			deleteStageResult: Stage{ID: 11, BoardID: 7, Title: "Doing"},
		}
		store := &fakeStore{transactionStore: transaction}
		got, err := NewService(store).DeleteStage(context.Background(), "work", "doing")
		if code, _ := apperr.CodeOf(err); code != apperr.Conflict {
			t.Fatalf("DeleteStage() error = %v, want conflict", err)
		}
		if !reflect.DeepEqual(got, StageDeletion{}) || store.transactionCalls != 1 ||
			!reflect.DeepEqual(transaction.stageOccupiedIDs, []int64{11}) ||
			transaction.clearDefersCalls != 0 || transaction.deleteStageID != 0 {
			t.Errorf("DeleteStage() result/delegation = %#v/%#v, want conflict before clear and delete", got, transaction)
		}
	})
}

func TestDeleteStageClearsDefersBeforeDeleting(t *testing.T) {
	t.Parallel()

	cleared := []task.Task{{ID: 31, Title: "first defer"}, {ID: 32, Title: "second defer"}}
	board := Board{ID: 7, Title: "Work"}
	stage := Stage{ID: 11, BoardID: board.ID, Title: "Doing"}
	transaction := &fakeStore{
		findBoards:        map[string]Board{"work": board},
		findStages:        map[string]Stage{"doing": stage},
		clearDefersResult: cleared,
		deleteStageResult: stage,
	}
	store := &fakeStore{transactionStore: transaction}
	service := NewService(store)
	service.now = func() time.Time {
		return time.Date(2026, time.August, 9, 1, 2, 3, 456000000, time.UTC)
	}

	got, err := service.DeleteStage(context.Background(), "work", "doing")
	if err != nil {
		t.Fatalf("DeleteStage() error = %v", err)
	}
	want := StageDeletion{Board: board, Stage: stage, ClearedDefers: cleared}
	if !reflect.DeepEqual(got, want) || store.transactionCalls != 1 || store.clearDefersCalls != 0 ||
		transaction.clearDefersCalls != 1 || transaction.clearDefersStageID != stage.ID ||
		transaction.clearDefersTimestamp != "2026-08-09T01:02:03.456Z" ||
		transaction.deleteStageBoardID != board.ID || transaction.deleteStageID != stage.ID ||
		!callPrecedes(transaction.calls, "stage occupied", "clear defers") ||
		!callPrecedes(transaction.calls, "clear defers", "delete stage") {
		t.Errorf("DeleteStage() result/delegation = %#v/%#v, want transactional occupancy, clear, and delete ordering", got, transaction)
	}
}

func TestDeleteStageClearFailureAbortsDelete(t *testing.T) {
	t.Parallel()

	clearError := apperr.New(apperr.Internal, "clear task stage defers", errors.New("write failed"))
	transaction := &fakeStore{
		findBoards:       map[string]Board{"work": {ID: 7, Title: "Work"}},
		findStages:       map[string]Stage{"doing": {ID: 11, BoardID: 7, Title: "Doing"}},
		clearDefersError: clearError,
	}
	store := &fakeStore{transactionStore: transaction}

	got, err := NewService(store).DeleteStage(context.Background(), "work", "doing")
	if !errors.Is(err, clearError) {
		t.Fatalf("DeleteStage() error = %v, want preserved clear error %v", err, clearError)
	}
	if !reflect.DeepEqual(got, StageDeletion{}) || transaction.clearDefersCalls != 1 ||
		transaction.deleteStageID != 0 {
		t.Errorf("DeleteStage() result/delegation = %#v/%#v, want zero result and no delete after clear failure", got, transaction)
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
	if !reflect.DeepEqual(got, want) || store.transactionCalls != 1 || transaction.deleteBoardID != 7 ||
		!reflect.DeepEqual(transaction.boardOccupiedIDs, []int64{7}) ||
		!callPrecedes(transaction.calls, "board occupied", "delete board") ||
		!callPrecedes(transaction.calls, "list stages", "delete board") {
		t.Errorf("Delete() result/delegation = %#v/%#v, want occupancy check and snapshot before deletion", got, transaction)
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
	if !reflect.DeepEqual(got, want) || transaction.renameStageBoardID != 7 || transaction.renameStageID != 12 || transaction.renameStageTitle != "In Progress" {
		t.Errorf("RenameStage() = %#v/%#v, want stored context and old spelling", got, transaction)
	}
}
