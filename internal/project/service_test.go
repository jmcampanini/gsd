package project

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

type recordingStore struct {
	addCalls             int
	addFields            CreateFields
	addTimestamp         string
	addError             error
	findCalls            int
	findID               int64
	findResult           Project
	findResults          []Project
	findError            error
	findErrors           []error
	listCalls            int
	listOptions          ListOptions
	listResult           []Project
	listError            error
	areaExistsCalls      int
	areaExistsID         int64
	areaExistsError      error
	findAreaCalls        int
	findAreaID           int64
	findAreaResult       AreaReference
	findAreaError        error
	findBoardCalls       int
	findBoardTitle       string
	findBoardResult      BoardReference
	findBoardError       error
	findFirstStageCalls  int
	findFirstStageBoard  int64
	findFirstStageResult *StageReference
	findFirstStageError  error
	findStageCalls       int
	findStageBoard       int64
	findStageTitle       string
	findStageResult      StageReference
	findStageError       error
	findStageByIDCalls   int
	findStageByID        int64
	findStageByIDResult  StageReference
	findStageByIDError   error
	editCalls            int
	editID               int64
	editFields           UpdateFields
	editTimestamp        string
	editResult           Project
	editError            error
	moveStageCalls       int
	moveStageProjectID   int64
	moveStageID          int64
	moveStagePlacement   domain.Placement
	moveStageTimestamp   string
	moveStageResult      Project
	moveStageError       error
	reorderCalls         int
	reorderID            int64
	reorderPlacement     domain.Placement
	reorderTimestamp     string
	reorderResult        Project
	reorderError         error
	resolveCalls         int
	resolveID            int64
	resolveExit          Exit
	resolveTimestamp     string
	resolveResult        Project
	resolveError         error
	cancelOpenTasksCalls int
	cancelProjectID      int64
	cancelTimestamp      string
	cancelResult         []task.Task
	cancelError          error
	reopenCalls          int
	reopenID             int64
	reopenTimestamp      string
	reopenResult         Project
	reopenError          error
	deleteCalls          int
	deleteID             int64
	deleteResult         Project
	deleteError          error
	deleteTasksCalls     int
	deleteTasksProjectID int64
	deleteTasksResult    []task.Task
	deleteTasksError     error
	resolveTagsCalls     int
	resolveTagNames      []string
	resolveTagsResult    []tag.Tag
	resolveTagsError     error
	attachTagsCalls      int
	attachProjectID      int64
	attachedTags         []tag.Tag
	attachTagsError      error
	detachTagsCalls      int
	detachProjectID      int64
	detachedTags         []tag.Tag
	detachTagsError      error
	transactionCalls     int
	transactionStore     Transaction
	transactionError     error
	readTransactionCalls int
	readTransactionStore Transaction
	readTransactionError error
}

func (r *recordingStore) Add(
	_ context.Context,
	fields CreateFields,
	timestamp string,
) (Project, error) {
	r.addCalls++
	r.addFields = fields
	r.addTimestamp = timestamp

	return Project{
		ID:        1,
		AreaID:    fields.AreaID,
		StageID:   fields.StageID,
		Title:     fields.Title,
		Note:      fields.Note,
		Status:    string(ListStatusOpen),
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}, r.addError
}

func (r *recordingStore) Find(_ context.Context, id int64) (Project, error) {
	index := r.findCalls
	r.findCalls++
	r.findID = id

	result := r.findResult
	if index < len(r.findResults) {
		result = r.findResults[index]
	}
	if result.ID == 0 {
		result.ID = id
	}
	err := r.findError
	if index < len(r.findErrors) {
		err = r.findErrors[index]
	}
	return result, err
}

func (r *recordingStore) List(_ context.Context, options ListOptions) ([]Project, error) {
	r.listCalls++
	r.listOptions = options
	return r.listResult, r.listError
}

func (r *recordingStore) AreaExists(_ context.Context, id int64) error {
	r.areaExistsCalls++
	r.areaExistsID = id
	return r.areaExistsError
}

func (r *recordingStore) FindArea(_ context.Context, id int64) (AreaReference, error) {
	r.findAreaCalls++
	r.findAreaID = id
	if r.findAreaResult.ID != 0 {
		return r.findAreaResult, r.findAreaError
	}
	return AreaReference{ID: id}, r.findAreaError
}

func (r *recordingStore) FindBoard(_ context.Context, title string) (BoardReference, error) {
	r.findBoardCalls++
	r.findBoardTitle = title
	if r.findBoardResult.ID != 0 {
		return r.findBoardResult, r.findBoardError
	}
	return BoardReference{ID: 1, Title: title}, r.findBoardError
}

func (r *recordingStore) FindFirstStage(_ context.Context, boardID int64) (*StageReference, error) {
	r.findFirstStageCalls++
	r.findFirstStageBoard = boardID
	return r.findFirstStageResult, r.findFirstStageError
}

func (r *recordingStore) FindStage(
	_ context.Context,
	boardID int64,
	title string,
) (StageReference, error) {
	r.findStageCalls++
	r.findStageBoard = boardID
	r.findStageTitle = title
	return r.findStageResult, r.findStageError
}

func (r *recordingStore) FindStageByID(_ context.Context, id int64) (StageReference, error) {
	r.findStageByIDCalls++
	r.findStageByID = id
	if r.findStageByIDResult.ID != 0 {
		return r.findStageByIDResult, r.findStageByIDError
	}
	return StageReference{ID: id}, r.findStageByIDError
}

func (r *recordingStore) Edit(
	_ context.Context,
	id int64,
	fields UpdateFields,
	timestamp string,
) (Project, error) {
	r.editCalls++
	r.editID = id
	r.editFields = fields
	r.editTimestamp = timestamp

	if r.editResult.ID != 0 {
		return r.editResult, r.editError
	}
	return Project{ID: id, UpdatedAt: timestamp}, r.editError
}

func (r *recordingStore) MoveStage(
	_ context.Context,
	projectID int64,
	stageID int64,
	placement domain.Placement,
	timestamp string,
) (Project, error) {
	r.moveStageCalls++
	r.moveStageProjectID = projectID
	r.moveStageID = stageID
	r.moveStagePlacement = placement
	r.moveStageTimestamp = timestamp
	return r.moveStageResult, r.moveStageError
}

func (r *recordingStore) Reorder(
	_ context.Context,
	id int64,
	placement domain.Placement,
	timestamp string,
) (Project, error) {
	r.reorderCalls++
	r.reorderID = id
	r.reorderPlacement = placement
	r.reorderTimestamp = timestamp
	return r.reorderResult, r.reorderError
}

func (r *recordingStore) Resolve(
	_ context.Context,
	id int64,
	exit Exit,
	timestamp string,
) (Project, error) {
	r.resolveCalls++
	r.resolveID = id
	r.resolveExit = exit
	r.resolveTimestamp = timestamp

	if r.resolveResult.ID == 0 {
		r.resolveResult = Project{ID: id, UpdatedAt: timestamp}
	}
	return r.resolveResult, r.resolveError
}

func (r *recordingStore) CancelOpenTasks(
	_ context.Context,
	projectID int64,
	timestamp string,
) ([]task.Task, error) {
	r.cancelOpenTasksCalls++
	r.cancelProjectID = projectID
	r.cancelTimestamp = timestamp
	return r.cancelResult, r.cancelError
}

func (r *recordingStore) Reopen(
	_ context.Context,
	id int64,
	timestamp string,
) (Project, error) {
	r.reopenCalls++
	r.reopenID = id
	r.reopenTimestamp = timestamp

	if r.reopenResult.ID == 0 {
		r.reopenResult = Project{ID: id, UpdatedAt: timestamp}
	}
	return r.reopenResult, r.reopenError
}

func (r *recordingStore) Delete(_ context.Context, id int64) (Project, error) {
	r.deleteCalls++
	r.deleteID = id

	if r.deleteResult.ID == 0 {
		r.deleteResult = Project{ID: id}
	}
	return r.deleteResult, r.deleteError
}

func (r *recordingStore) DeleteTasks(
	_ context.Context,
	projectID int64,
) ([]task.Task, error) {
	r.deleteTasksCalls++
	r.deleteTasksProjectID = projectID
	return r.deleteTasksResult, r.deleteTasksError
}

func (r *recordingStore) ResolveTags(
	_ context.Context,
	names []string,
) ([]tag.Tag, error) {
	r.resolveTagsCalls++
	r.resolveTagNames = slices.Clone(names)
	return r.resolveTagsResult, r.resolveTagsError
}

func (r *recordingStore) AttachTags(
	_ context.Context,
	projectID int64,
	tags []tag.Tag,
) error {
	r.attachTagsCalls++
	r.attachProjectID = projectID
	r.attachedTags = slices.Clone(tags)
	return r.attachTagsError
}

func (r *recordingStore) DetachTags(
	_ context.Context,
	projectID int64,
	tags []tag.Tag,
) error {
	r.detachTagsCalls++
	r.detachProjectID = projectID
	r.detachedTags = slices.Clone(tags)
	return r.detachTagsError
}

func (r *recordingStore) WithinTransaction(
	ctx context.Context,
	operation func(Transaction) error,
) error {
	r.transactionCalls++
	if r.transactionError != nil {
		return r.transactionError
	}
	store := r.transactionStore
	if store == nil {
		store = r
	}
	return operation(store)
}

func (r *recordingStore) WithinReadTransaction(
	ctx context.Context,
	operation func(Transaction) error,
) error {
	r.readTransactionCalls++
	if r.readTransactionError != nil {
		return r.readTransactionError
	}
	store := r.readTransactionStore
	if store == nil {
		store = r
	}
	return operation(store)
}

func TestStoreErrorsPassThroughUnchanged(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	store := &recordingStore{
		addError:  storeError,
		findError: storeError,
		listError: storeError,
		editError: storeError,
	}
	service := NewService(store)
	title := "valid"
	operations := []struct {
		name  string
		apply func() error
	}{
		{name: "add", apply: func() error { _, err := service.Add(context.Background(), AddFields{Title: title}); return err }},
		{name: "list", apply: func() error {
			_, err := service.List(context.Background(), ListOptions{Status: ListStatusOpen})
			return err
		}},
		{name: "show", apply: func() error { _, err := service.Show(context.Background(), 1); return err }},
		{name: "edit", apply: func() error { _, err := service.Edit(context.Background(), 1, EditFields{Title: &title}); return err }},
	}
	for _, operation := range operations {
		if err := operation.apply(); !errors.Is(err, storeError) {
			t.Errorf("%s error = %v, want preserved store error %v", operation.name, err, storeError)
		}
	}
}

func TestAddPreservesTextAndDelegatesOneNormalizedTimestamp(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(
			2026,
			time.July,
			27,
			12,
			34,
			56,
			987654321,
			time.FixedZone("offset", -4*60*60),
		)
	}

	fields := AddFields{
		Title: "  Keep surrounding space  ",
		Note:  "line one\nline two\n",
	}
	created, err := service.Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.addCalls != 1 || !equalCreateFields(store.addFields, fields) {
		t.Errorf("store Add() calls/fields = %d/%#v, want 1/%#v", store.addCalls, store.addFields, fields)
	}
	if created.Title != fields.Title || created.Note != fields.Note {
		t.Errorf("Add() = %#v, want exact accepted text", created)
	}
	if store.transactionCalls != 0 {
		t.Errorf("transaction calls = %d, want direct Add() for no tags", store.transactionCalls)
	}
	if nowCalls != 1 || store.addTimestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf(
			"clock calls/timestamp = %d/%q, want one call and UTC milliseconds",
			nowCalls,
			store.addTimestamp,
		)
	}
}

func TestAddValidatesAndDelegatesArea(t *testing.T) {
	t.Parallel()

	areaID := int64(3)
	fields := AddFields{AreaID: &areaID, Title: "area project"}
	store := &recordingStore{}
	created, err := NewService(store).Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.addCalls != 1 || !equalCreateFields(store.addFields, fields) ||
		created.AreaID == nil || *created.AreaID != areaID {
		t.Errorf("store Add() calls/fields/result = %d/%#v/%#v, want 1/%#v/area %d", store.addCalls, store.addFields, created, fields, areaID)
	}

	invalidAreaID := int64(0)
	store = &recordingStore{}
	_, err = NewService(store).Add(context.Background(), AddFields{AreaID: &invalidAreaID, Title: "valid"})
	if errorCode(err) != apperr.InvalidArgument {
		t.Errorf("Add() error = %v, want invalid_argument", err)
	}
	if store.addCalls != 0 {
		t.Errorf("store Add() calls = %d, want 0", store.addCalls)
	}
}

func TestAddRejectsInvalidTextBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields AddFields
	}{
		{name: "blank title", fields: AddFields{Title: " \t\n"}},
		{name: "invalid title UTF-8", fields: AddFields{Title: string([]byte{0xff})}},
		{name: "invalid note UTF-8", fields: AddFields{Title: "valid", Note: string([]byte{0xff})}},
		{name: "blank board", fields: AddFields{Title: "valid", Board: func() *string { value := "\t"; return &value }()}},
		{name: "blank tag", fields: AddFields{Title: "valid", Tags: []string{"work", "\t"}}},
		{name: "invalid tag UTF-8", fields: AddFields{Title: "valid", Tags: []string{string([]byte{0xff})}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			_, err := NewService(store).Add(context.Background(), test.fields)
			if errorCode(err) != apperr.InvalidArgument {
				t.Errorf("Add() error = %v, want invalid_argument", err)
			}
			if store.addCalls != 0 || store.transactionCalls != 0 {
				t.Errorf("store Add()/transaction calls = %d/%d, want 0/0", store.addCalls, store.transactionCalls)
			}
		})
	}
}

func TestAddWithTagsNormalizesAndRefreshesWithinOneTransaction(t *testing.T) {
	t.Parallel()

	resolvedTags := []tag.Tag{{ID: 10, Title: "WORK"}, {ID: 11, Title: "É"}, {ID: 12, Title: "é"}}
	refreshed := Project{ID: 1, Title: "tagged", Tags: domain.TagNames{"WORK", "É", "é"}}
	transactionStore := &recordingStore{
		resolveTagsResult: resolvedTags,
		findResult:        refreshed,
	}
	store := &recordingStore{transactionStore: transactionStore}
	fields := AddFields{Title: "tagged", Tags: []string{"Work", "work", "É", "é"}}

	created, err := NewService(store).Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.transactionCalls != 1 || store.addCalls != 0 || store.resolveTagsCalls != 0 ||
		store.attachTagsCalls != 0 || store.findCalls != 0 {
		t.Errorf(
			"outer transaction/add/resolve/attach/find calls = %d/%d/%d/%d/%d, want 1/0/0/0/0",
			store.transactionCalls,
			store.addCalls,
			store.resolveTagsCalls,
			store.attachTagsCalls,
			store.findCalls,
		)
	}
	wantNames := []string{"Work", "É", "é"}
	if !slices.Equal(transactionStore.resolveTagNames, wantNames) {
		t.Errorf("normalized ResolveTags() names = %v, want %v", transactionStore.resolveTagNames, wantNames)
	}
	if transactionStore.addCalls != 1 || transactionStore.resolveTagsCalls != 1 ||
		transactionStore.attachTagsCalls != 1 || transactionStore.findCalls != 1 {
		t.Errorf("transaction-scoped calls = %#v, want one add, resolve, attach, and refresh", transactionStore)
	}
	if transactionStore.attachProjectID != 1 || !slices.Equal(transactionStore.attachedTags, resolvedTags) {
		t.Errorf(
			"AttachTags() project/tags = %d/%#v, want 1/%#v",
			transactionStore.attachProjectID,
			transactionStore.attachedTags,
			resolvedTags,
		)
	}
	if !reflect.DeepEqual(created, refreshed) {
		t.Errorf("Add() = %#v, want refreshed project %#v", created, refreshed)
	}
}

func TestAddResolvesBoardAndCreatesInItsFirstStage(t *testing.T) {
	t.Parallel()

	boardTitle := "SOFTWARE"
	first := StageReference{ID: 12, BoardID: 4, BoardTitle: "Software", Title: "Research"}
	transaction := &recordingStore{
		findBoardResult:      BoardReference{ID: 4, Title: "Software"},
		findFirstStageResult: &first,
	}
	store := &recordingStore{transactionStore: transaction}

	created, err := NewService(store).Add(context.Background(), AddFields{
		Board: &boardTitle,
		Title: "Boarded project",
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.transactionCalls != 1 || transaction.findBoardTitle != boardTitle ||
		transaction.findFirstStageBoard != 4 {
		t.Errorf("board resolution = outer transactions %d, board %q, first-stage board %d; want 1/%q/4",
			store.transactionCalls, transaction.findBoardTitle, transaction.findFirstStageBoard, boardTitle)
	}
	if transaction.addCalls != 1 || transaction.addFields.StageID == nil ||
		*transaction.addFields.StageID != first.ID {
		t.Errorf("Add() persistence = calls %d, fields %#v; want first stage %d", transaction.addCalls, transaction.addFields, first.ID)
	}
	if created.StageID == nil || *created.StageID != first.ID {
		t.Errorf("Add() = %#v, want project in first stage %d", created, first.ID)
	}
}

func TestAddRejectsUnknownAndStagelessBoardsBeforeCreating(t *testing.T) {
	t.Parallel()

	missing := apperr.New(apperr.NotFound, "no board missing", nil)
	tests := []struct {
		name      string
		configure func(*recordingStore)
		wantCode  apperr.Code
	}{
		{
			name: "unknown board",
			configure: func(store *recordingStore) {
				store.findBoardError = missing
			},
			wantCode: apperr.NotFound,
		},
		{
			name: "stageless board",
			configure: func(store *recordingStore) {
				store.findBoardResult = BoardReference{ID: 4, Title: "Empty"}
			},
			wantCode: apperr.Conflict,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transaction := &recordingStore{}
			test.configure(transaction)
			store := &recordingStore{transactionStore: transaction}
			boardTitle := "missing"
			_, err := NewService(store).Add(context.Background(), AddFields{Board: &boardTitle, Title: "project"})
			if errorCode(err) != test.wantCode {
				t.Errorf("Add() error = %v, want %s", err, test.wantCode)
			}
			if transaction.addCalls != 0 {
				t.Errorf("Add() persistence calls = %d, want 0", transaction.addCalls)
			}
		})
	}
}

func TestParseIDAndShowValidateProjectIDs(t *testing.T) {
	t.Parallel()

	parsed, err := ParseID("001")
	if err != nil || parsed != 1 {
		t.Fatalf("ParseID(001) = %d, %v, want 1, nil", parsed, err)
	}
	for _, value := range []string{"", "0", "-1", "+1", "1.0", "１２", "9223372036854775808"} {
		if got, parseErr := ParseID(value); errorCode(parseErr) != apperr.InvalidArgument {
			t.Errorf("ParseID(%q) = %d, %v, want invalid_argument", value, got, parseErr)
		}
	}

	store := &recordingStore{}
	if _, err := NewService(store).Show(context.Background(), 0); errorCode(err) != apperr.InvalidArgument {
		t.Errorf("Show(0) error = %v, want invalid_argument", err)
	}
	if store.findCalls != 0 {
		t.Errorf("store Find() calls = %d, want 0", store.findCalls)
	}
}

func TestParseListStatusAcceptsOnlySupportedStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []ListStatus{
		ListStatusOpen,
		ListStatusDone,
		ListStatusCancelled,
		ListStatusAll,
	} {
		parsed, err := ParseListStatus(string(status))
		if err != nil || parsed != status {
			t.Errorf("ParseListStatus(%q) = %q, %v, want %q, nil", status, parsed, err, status)
		}
	}
	if _, err := ParseListStatus("OPEN"); errorCode(err) != apperr.InvalidArgument {
		t.Errorf("ParseListStatus(OPEN) error = %v, want invalid_argument", err)
	}
}

func TestListValidatesDelegatesAndNormalizesNil(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	options := ListOptions{Status: ListStatusDone}
	listed, err := service.List(context.Background(), options)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("List() = %#v, want non-nil empty list", listed)
	}
	if store.listCalls != 1 || store.listOptions != options {
		t.Errorf(
			"store List() calls/options = %d/%#v, want 1/%#v",
			store.listCalls,
			store.listOptions,
			options,
		)
	}

	if _, err := service.List(
		context.Background(),
		ListOptions{Status: ListStatus("invalid")},
	); errorCode(err) != apperr.InvalidArgument {
		t.Errorf("List(invalid) error = %v, want invalid_argument", err)
	}
	if store.listCalls != 1 {
		t.Errorf("store List() calls after invalid request = %d, want 1", store.listCalls)
	}
}

func TestListValidatesAndDelegatesArea(t *testing.T) {
	t.Parallel()

	areaID := int64(3)
	options := ListOptions{Status: ListStatusDone, AreaID: &areaID}
	store := &recordingStore{}
	if _, err := NewService(store).List(context.Background(), options); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.readTransactionCalls != 1 || store.areaExistsCalls != 1 ||
		store.areaExistsID != areaID || store.listCalls != 1 || store.listOptions != options {
		t.Errorf(
			"store read/area/ID/list/options = %d/%d/%d/%d/%#v, want 1/1/%d/1/%#v",
			store.readTransactionCalls,
			store.areaExistsCalls,
			store.areaExistsID,
			store.listCalls,
			store.listOptions,
			areaID,
			options,
		)
	}

	invalidAreaID := int64(0)
	store = &recordingStore{}
	_, err := NewService(store).List(context.Background(), ListOptions{Status: ListStatusOpen, AreaID: &invalidAreaID})
	if errorCode(err) != apperr.InvalidArgument {
		t.Errorf("List() error = %v, want invalid_argument", err)
	}
	if store.readTransactionCalls != 0 || store.areaExistsCalls != 0 || store.listCalls != 0 {
		t.Errorf(
			"store read/area/list calls = %d/%d/%d, want 0/0/0",
			store.readTransactionCalls,
			store.areaExistsCalls,
			store.listCalls,
		)
	}
}

func TestListReturnsUnknownAreaErrorBeforeListing(t *testing.T) {
	t.Parallel()

	areaID := int64(999)
	missingArea := apperr.New(apperr.NotFound, "no area 999", errors.New("missing area"))
	store := &recordingStore{areaExistsError: missingArea}
	_, err := NewService(store).List(context.Background(), ListOptions{
		Status: ListStatusAll,
		AreaID: &areaID,
	})
	if !errors.Is(err, missingArea) {
		t.Fatalf("List(missing area) error = %v, want preserved %v", err, missingArea)
	}
	if store.readTransactionCalls != 1 || store.areaExistsCalls != 1 || store.listCalls != 0 {
		t.Errorf(
			"store read/area/list calls = %d/%d/%d, want 1/1/0",
			store.readTransactionCalls,
			store.areaExistsCalls,
			store.listCalls,
		)
	}
}

func TestEditValidatesAndDelegatesOneNormalizedTimestamp(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(
			2026,
			time.July,
			27,
			12,
			34,
			56,
			987654321,
			time.FixedZone("offset", -4*60*60),
		)
	}

	title := "  Revised title  "
	note := "line one\nline two\n"
	fields := EditFields{Title: &title, Note: &note}
	edited, err := service.Edit(context.Background(), 7, fields)
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if store.editCalls != 1 || store.editID != 7 || store.editFields.Title == nil ||
		*store.editFields.Title != title || store.editFields.Note == nil || *store.editFields.Note != note {
		t.Errorf(
			"store Edit() calls/ID/fields = %d/%d/%#v, want 1/7/%#v",
			store.editCalls,
			store.editID,
			store.editFields,
			fields,
		)
	}
	if nowCalls != 1 || store.editTimestamp != "2026-07-27T16:34:56.987Z" ||
		edited.Project.UpdatedAt != store.editTimestamp {
		t.Errorf(
			"clock calls/timestamp = %d/%q, want one call and UTC milliseconds",
			nowCalls,
			store.editTimestamp,
		)
	}
}

func TestEditValidatesAndDelegatesAreaIntent(t *testing.T) {
	t.Parallel()

	areaID := int64(3)
	accepted := []EditFields{
		{Area: AreaChange{Set: &areaID}},
		{Area: AreaChange{Clear: true}},
	}
	for _, fields := range accepted {
		store := &recordingStore{}
		if _, err := NewService(store).Edit(context.Background(), 7, fields); err != nil {
			t.Fatalf("Edit(%#v) error = %v", fields, err)
		}
		if store.editCalls != 1 || store.editFields.Area != fields.Area {
			t.Errorf("store Edit() calls/area = %d/%#v, want 1/%#v", store.editCalls, store.editFields.Area, fields.Area)
		}
	}

	invalidAreaID := int64(0)
	boardTitle := "software"
	invalid := []EditFields{
		{Area: AreaChange{Set: &invalidAreaID}},
		{Area: AreaChange{Set: &areaID, Clear: true}},
		{Board: BoardChange{Set: &boardTitle, Clear: true}},
	}
	for _, fields := range invalid {
		store := &recordingStore{}
		_, err := NewService(store).Edit(context.Background(), 7, fields)
		if errorCode(err) != apperr.InvalidArgument {
			t.Errorf("Edit(%#v) error = %v, want invalid_argument", fields, err)
		}
		if store.editCalls != 0 {
			t.Errorf("store Edit() calls = %d, want 0", store.editCalls)
		}
	}
}

func TestEditRejectsInvalidRequestBeforePersistence(t *testing.T) {
	t.Parallel()

	validTitle := "valid"
	blankTitle := " \t\n"
	invalidTitle := string([]byte{0xff})
	invalidNote := string([]byte{0xff})
	tests := []struct {
		name   string
		id     int64
		fields EditFields
	}{
		{name: "nonpositive ID", fields: EditFields{Title: &validTitle}},
		{name: "no fields", id: 1},
		{name: "blank title", id: 1, fields: EditFields{Title: &blankTitle}},
		{name: "invalid title UTF-8", id: 1, fields: EditFields{Title: &invalidTitle}},
		{name: "invalid note UTF-8", id: 1, fields: EditFields{Note: &invalidNote}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			_, err := NewService(store).Edit(context.Background(), test.id, test.fields)
			if errorCode(err) != apperr.InvalidArgument {
				t.Errorf("Edit() error = %v, want invalid_argument", err)
			}
			if store.editCalls != 0 {
				t.Errorf("store Edit() calls = %d, want 0", store.editCalls)
			}
		})
	}
}

func TestEditBoardMembershipSwitchesRestatesAndClears(t *testing.T) {
	t.Parallel()

	t.Run("switch enters first stage", func(t *testing.T) {
		t.Parallel()

		currentStageID := int64(10)
		first := StageReference{ID: 20, BoardID: 2, BoardTitle: "New", Title: "Intake"}
		want := Project{ID: 7, StageID: &first.ID, Status: string(ListStatusOpen)}
		transaction := &recordingStore{
			findResult:           Project{ID: 7, StageID: &currentStageID, Status: string(ListStatusOpen)},
			findStageByIDResult:  StageReference{ID: currentStageID, BoardID: 1, BoardTitle: "Old", Title: "Doing"},
			findBoardResult:      BoardReference{ID: 2, Title: "New"},
			findFirstStageResult: &first,
			editResult:           want,
		}
		store := &recordingStore{transactionStore: transaction}
		boardTitle := "new"

		got, err := NewService(store).Edit(context.Background(), 7, EditFields{Board: BoardChange{Set: &boardTitle}})
		if err != nil {
			t.Fatalf("Edit() error = %v", err)
		}
		if transaction.editCalls != 1 || transaction.editFields.Stage.Set == nil ||
			*transaction.editFields.Stage.Set != first.ID || transaction.editFields.Stage.Clear {
			t.Errorf("Edit() persistence = calls %d, fields %#v; want set stage %d", transaction.editCalls, transaction.editFields, first.ID)
		}
		if !reflect.DeepEqual(got.Project, want) || got.Location == nil ||
			got.Location.BoardTitle != first.BoardTitle || got.Location.StageTitle != first.Title ||
			got.ClearedDefers == nil {
			t.Errorf("Edit() = %#v, want switched project, stored location, and empty defer list", got)
		}
	})

	t.Run("restating board preserves stage and position", func(t *testing.T) {
		t.Parallel()

		stageID := int64(10)
		position := int64(3)
		current := Project{ID: 7, StageID: &stageID, StagePosition: &position, Status: string(ListStatusOpen)}
		stage := StageReference{ID: stageID, BoardID: 2, BoardTitle: "Software", Title: "Doing"}
		transaction := &recordingStore{
			findResult:          current,
			findStageByIDResult: stage,
			findBoardResult:     BoardReference{ID: 2, Title: "Software"},
		}
		store := &recordingStore{transactionStore: transaction}
		boardTitle := "SOFTWARE"

		got, err := NewService(store).Edit(context.Background(), 7, EditFields{Board: BoardChange{Set: &boardTitle}})
		if err != nil {
			t.Fatalf("Edit() error = %v", err)
		}
		if transaction.findFirstStageCalls != 0 || transaction.editCalls != 0 {
			t.Errorf("first-stage/edit calls = %d/%d, want 0/0 for redundant membership", transaction.findFirstStageCalls, transaction.editCalls)
		}
		if !reflect.DeepEqual(got.Project, current) || got.Location == nil || got.Location.StageTitle != stage.Title {
			t.Errorf("Edit() = %#v, want unchanged project and current location", got)
		}
	})

	t.Run("stageless destination conflicts", func(t *testing.T) {
		t.Parallel()

		transaction := &recordingStore{
			findResult:      Project{ID: 7, Status: string(ListStatusOpen)},
			findBoardResult: BoardReference{ID: 2, Title: "Empty"},
		}
		store := &recordingStore{transactionStore: transaction}
		boardTitle := "empty"

		_, err := NewService(store).Edit(context.Background(), 7, EditFields{Board: BoardChange{Set: &boardTitle}})
		if errorCode(err) != apperr.Conflict {
			t.Errorf("Edit() error = %v, want conflict", err)
		}
		if transaction.editCalls != 0 {
			t.Errorf("Edit() persistence calls = %d, want 0", transaction.editCalls)
		}
	})

	t.Run("clear membership", func(t *testing.T) {
		t.Parallel()

		stageID := int64(10)
		current := Project{ID: 7, StageID: &stageID, Status: string(ListStatusOpen)}
		want := Project{ID: 7, Status: string(ListStatusOpen)}
		transaction := &recordingStore{
			findResult:          current,
			findStageByIDResult: StageReference{ID: stageID, BoardID: 2, BoardTitle: "Software", Title: "Doing"},
			editResult:          want,
		}
		store := &recordingStore{transactionStore: transaction}

		got, err := NewService(store).Edit(context.Background(), 7, EditFields{Board: BoardChange{Clear: true}})
		if err != nil {
			t.Fatalf("Edit() error = %v", err)
		}
		if transaction.editCalls != 1 || !transaction.editFields.Stage.Clear || transaction.editFields.Stage.Set != nil {
			t.Errorf("Edit() persistence = calls %d, fields %#v; want stage clear", transaction.editCalls, transaction.editFields)
		}
		if !reflect.DeepEqual(got.Project, want) || got.Location != nil {
			t.Errorf("Edit() = %#v, want unboarded project without location", got)
		}
	})
}

func TestBoardMembershipRejectsResolvedAndArchivedProjects(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-01-01T00:00:00.000Z"
	tests := []struct {
		name      string
		current   Project
		configure func(*recordingStore)
		assert    func(*testing.T, error)
	}{
		{
			name:    "resolved project",
			current: Project{ID: 7, Status: string(ListStatusDone)},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var marker *ResolvedProjectsError
				if !errors.As(err, &marker) || !reflect.DeepEqual(marker.IDs, []int64{7}) {
					t.Errorf("Edit() error = %v, want resolved project marker for 7", err)
				}
			},
		},
		{
			name: "archived governing area",
			current: func() Project {
				areaID := int64(3)
				return Project{ID: 7, AreaID: &areaID, Status: string(ListStatusOpen)}
			}(),
			configure: func(store *recordingStore) {
				store.findAreaResult = AreaReference{ID: 3, ArchivedAt: &archivedAt}
			},
			assert: func(t *testing.T, err error) {
				t.Helper()
				var marker *ArchivedAreasError
				if !errors.As(err, &marker) || !reflect.DeepEqual(marker.IDs, []int64{3}) {
					t.Errorf("Edit() error = %v, want archived area marker for 3", err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stageID := int64(10)
			test.current.StageID = &stageID
			transaction := &recordingStore{
				findResult:          test.current,
				findStageByIDResult: StageReference{ID: stageID, BoardID: 1, BoardTitle: "Software", Title: "Doing"},
			}
			if test.configure != nil {
				test.configure(transaction)
			}
			store := &recordingStore{transactionStore: transaction}
			_, err := NewService(store).Edit(context.Background(), 7, EditFields{Board: BoardChange{Clear: true}})
			if errorCode(err) != apperr.Conflict {
				t.Fatalf("Edit() error = %v, want conflict", err)
			}
			test.assert(t, err)
			if transaction.editCalls != 0 {
				t.Errorf("Edit() persistence calls = %d, want 0", transaction.editCalls)
			}
		})
	}
}

func TestMoveAcrossStagesDelegatesAppendAndExplicitPlacement(t *testing.T) {
	t.Parallel()

	currentStageID := int64(10)
	destinationStageID := int64(20)
	current := Project{ID: 7, StageID: &currentStageID, Status: string(ListStatusOpen)}
	destination := StageReference{ID: destinationStageID, BoardID: 1, BoardTitle: "Software", Title: "Doing"}

	t.Run("bare move appends", func(t *testing.T) {
		t.Parallel()

		want := Project{ID: 7, StageID: &destinationStageID, Status: string(ListStatusOpen)}
		transaction := &recordingStore{
			findResult:          current,
			findStageByIDResult: StageReference{ID: currentStageID, BoardID: 1, BoardTitle: "Software", Title: "Research"},
			findStageResult:     destination,
			moveStageResult:     want,
		}
		store := &recordingStore{transactionStore: transaction}
		service := NewService(store)
		service.now = func() time.Time { return time.Date(2026, time.August, 8, 1, 2, 3, 456000000, time.UTC) }

		got, err := service.Move(context.Background(), 7, "doing", nil)
		if err != nil {
			t.Fatalf("Move() error = %v", err)
		}
		if transaction.moveStageCalls != 1 || transaction.moveStageProjectID != 7 ||
			transaction.moveStageID != destinationStageID || transaction.moveStagePlacement != (domain.Placement{}) ||
			transaction.moveStageTimestamp != "2026-08-08T01:02:03.456Z" {
			t.Errorf("MoveStage() delegation = %#v, want project 7 append to stage %d with one timestamp", transaction, destinationStageID)
		}
		if !reflect.DeepEqual(got.Project, want) || got.StageTitle != destination.Title {
			t.Errorf("Move() = %#v, want moved project and stored destination title", got)
		}
	})

	t.Run("explicit placement is honored", func(t *testing.T) {
		t.Parallel()

		referenceStageID := destinationStageID
		placement := domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 8}
		transaction := &recordingStore{
			findResults: []Project{
				current,
				{ID: 8, StageID: &referenceStageID, Status: string(ListStatusOpen)},
			},
			findStageByIDResult: StageReference{ID: currentStageID, BoardID: 1, BoardTitle: "Software", Title: "Research"},
			findStageResult:     destination,
			moveStageResult:     Project{ID: 7, StageID: &destinationStageID},
		}
		store := &recordingStore{transactionStore: transaction}

		_, err := NewService(store).Move(context.Background(), 7, "Doing", &placement)
		if err != nil {
			t.Fatalf("Move() error = %v", err)
		}
		if transaction.moveStageCalls != 1 || transaction.moveStageID != destinationStageID ||
			transaction.moveStagePlacement != placement {
			t.Errorf("MoveStage() placement = calls %d, stage %d, placement %#v; want 1/%d/%#v",
				transaction.moveStageCalls, transaction.moveStageID, transaction.moveStagePlacement, destinationStageID, placement)
		}
	})
}

func TestMoveSameStageBareNoOpsAndPlacementReorders(t *testing.T) {
	t.Parallel()

	stageID := int64(10)
	current := Project{ID: 7, StageID: &stageID, StagePosition: func() *int64 { value := int64(2); return &value }(), Status: string(ListStatusOpen)}
	stage := StageReference{ID: stageID, BoardID: 1, BoardTitle: "Software", Title: "Doing"}

	t.Run("bare no-op", func(t *testing.T) {
		t.Parallel()

		transaction := &recordingStore{findResult: current, findStageByIDResult: stage, findStageResult: stage}
		store := &recordingStore{transactionStore: transaction}
		service := NewService(store)
		nowCalls := 0
		service.now = func() time.Time { nowCalls++; return time.Time{} }

		got, err := service.Move(context.Background(), 7, "doing", nil)
		if err != nil {
			t.Fatalf("Move() error = %v", err)
		}
		if transaction.moveStageCalls != 0 || nowCalls != 0 {
			t.Errorf("MoveStage()/clock calls = %d/%d, want 0/0", transaction.moveStageCalls, nowCalls)
		}
		if !reflect.DeepEqual(got.Project, current) || got.StageTitle != stage.Title {
			t.Errorf("Move() = %#v, want unchanged project in stored stage", got)
		}
	})

	t.Run("placement reorders", func(t *testing.T) {
		t.Parallel()

		placement := domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 8}
		transaction := &recordingStore{
			findResults:         []Project{current, {ID: 8, StageID: &stageID}},
			findStageByIDResult: stage,
			findStageResult:     stage,
			moveStageResult:     Project{ID: 7, StageID: &stageID},
		}
		store := &recordingStore{transactionStore: transaction}

		_, err := NewService(store).Move(context.Background(), 7, "doing", &placement)
		if err != nil {
			t.Fatalf("Move() error = %v", err)
		}
		if transaction.moveStageCalls != 1 || transaction.moveStageID != stageID ||
			transaction.moveStagePlacement != placement {
			t.Errorf("MoveStage() = calls %d, stage %d, placement %#v; want 1/%d/%#v",
				transaction.moveStageCalls, transaction.moveStageID, transaction.moveStagePlacement, stageID, placement)
		}
	})
}

func TestMoveRejectsUnknownStageAndForeignPlacementReference(t *testing.T) {
	t.Parallel()

	stageID := int64(10)
	current := Project{ID: 7, StageID: &stageID, Status: string(ListStatusOpen)}
	currentStage := StageReference{ID: stageID, BoardID: 1, BoardTitle: "Software", Title: "Research"}

	t.Run("unknown stage", func(t *testing.T) {
		t.Parallel()

		transaction := &recordingStore{
			findResult:          current,
			findStageByIDResult: currentStage,
			findStageError:      apperr.New(apperr.NotFound, "no stage shipping", nil),
		}
		store := &recordingStore{transactionStore: transaction}
		_, err := NewService(store).Move(context.Background(), 7, "shipping", nil)
		if errorCode(err) != apperr.NotFound {
			t.Errorf("Move() error = %v, want not_found", err)
		}
		if transaction.moveStageCalls != 0 {
			t.Errorf("MoveStage() calls = %d, want 0", transaction.moveStageCalls)
		}
	})

	for _, test := range []struct {
		name        string
		destination StageReference
	}{
		{name: "same-stage reference", destination: currentStage},
		{name: "cross-stage reference", destination: StageReference{ID: 20, BoardID: 1, BoardTitle: "Software", Title: "Doing"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			foreignStageID := int64(99)
			placement := domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 8}
			transaction := &recordingStore{
				findResults:         []Project{current, {ID: 8, StageID: &foreignStageID}},
				findStageByIDResult: currentStage,
				findStageResult:     test.destination,
			}
			store := &recordingStore{transactionStore: transaction}
			_, err := NewService(store).Move(context.Background(), 7, test.destination.Title, &placement)
			if errorCode(err) != apperr.InvalidArgument {
				t.Errorf("Move() error = %v, want invalid_argument", err)
			}
			if transaction.moveStageCalls != 0 {
				t.Errorf("MoveStage() calls = %d, want 0", transaction.moveStageCalls)
			}
		})
	}
}

func TestMoveRejectsResolvedAndArchivedProjects(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-01-01T00:00:00.000Z"
	tests := []struct {
		name      string
		current   Project
		configure func(*recordingStore)
	}{
		{name: "resolved", current: Project{ID: 7, Status: string(ListStatusDone)}},
		{
			name: "archived area",
			current: func() Project {
				areaID := int64(3)
				return Project{ID: 7, AreaID: &areaID, Status: string(ListStatusOpen)}
			}(),
			configure: func(store *recordingStore) {
				store.findAreaResult = AreaReference{ID: 3, ArchivedAt: &archivedAt}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stageID := int64(10)
			test.current.StageID = &stageID
			stage := StageReference{ID: stageID, BoardID: 1, BoardTitle: "Software", Title: "Doing"}
			transaction := &recordingStore{findResult: test.current, findStageByIDResult: stage, findStageResult: stage}
			if test.configure != nil {
				test.configure(transaction)
			}
			store := &recordingStore{transactionStore: transaction}
			_, err := NewService(store).Move(context.Background(), 7, "doing", nil)
			if errorCode(err) != apperr.Conflict {
				t.Errorf("Move() error = %v, want conflict", err)
			}
			if transaction.moveStageCalls != 0 {
				t.Errorf("MoveStage() calls = %d, want 0", transaction.moveStageCalls)
			}
		})
	}
}

func TestReorderRejectsInvalidInputBeforeClockOrStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        int64
		placement domain.Placement
	}{
		{name: "moved ID", placement: domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 3}},
		{name: "missing relative reference", id: 7, placement: domain.Placement{Anchor: domain.PlacementAfter}},
		{name: "nonpositive relative reference", id: 7, placement: domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: -1}},
		{name: "unknown anchor", id: 7, placement: domain.Placement{Anchor: domain.PlacementAnchor("middle")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			service := NewService(store)
			nowCalls := 0
			service.now = func() time.Time {
				nowCalls++
				return time.Time{}
			}

			_, err := service.Reorder(context.Background(), test.id, test.placement)
			if errorCode(err) != apperr.InvalidArgument {
				t.Errorf("Reorder() error = %v, want invalid_argument", err)
			}
			if store.reorderCalls != 0 || nowCalls != 0 {
				t.Errorf("store/clock calls = %d/%d, want 0/0", store.reorderCalls, nowCalls)
			}
		})
	}
}

func TestReorderDelegatesExactPlacementTimestampAndStoreError(t *testing.T) {
	t.Parallel()

	placement := domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 11}
	want := Project{ID: 7, Title: "moved", Position: 2}
	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	store := &recordingStore{reorderResult: want}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	got, err := service.Reorder(context.Background(), 7, placement)
	if err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) || store.reorderCalls != 1 || store.reorderID != 7 ||
		store.reorderPlacement != placement {
		t.Errorf("Reorder() result/delegation = %#v/%#v, want %#v and exact intent", got, store, want)
	}
	if nowCalls != 1 || store.reorderTimestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want 1/UTC milliseconds", nowCalls, store.reorderTimestamp)
	}

	store.reorderError = storeError
	if _, err := service.Reorder(context.Background(), 7, placement); !errors.Is(err, storeError) {
		t.Errorf("Reorder() error = %v, want preserved %v", err, storeError)
	}
}

func TestLifecycleRejectsInvalidIDsAndExitsBeforeDelegation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{
			name: "resolve ID",
			apply: func(service *Service) error {
				_, err := service.Resolve(context.Background(), 0, ExitDone)
				return err
			},
		},
		{
			name: "resolve exit",
			apply: func(service *Service) error {
				_, err := service.Resolve(context.Background(), 1, Exit("open"))
				return err
			},
		},
		{
			name: "reopen ID",
			apply: func(service *Service) error {
				_, err := service.Reopen(context.Background(), 0)
				return err
			},
		},
		{
			name: "nonrecursive delete ID",
			apply: func(service *Service) error {
				_, err := service.Delete(context.Background(), 0, false)
				return err
			},
		},
		{
			name: "recursive delete ID",
			apply: func(service *Service) error {
				_, err := service.Delete(context.Background(), 0, true)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			service := NewService(store)
			nowCalls := 0
			service.now = func() time.Time {
				nowCalls++
				return time.Time{}
			}

			if err := test.apply(service); errorCode(err) != apperr.InvalidArgument {
				t.Errorf("lifecycle error = %v, want invalid_argument", err)
			}
			storeCalls := store.resolveCalls + store.cancelOpenTasksCalls + store.reopenCalls +
				store.deleteCalls + store.deleteTasksCalls + store.transactionCalls
			if storeCalls != 0 || nowCalls != 0 {
				t.Errorf("store/clock calls = %d/%d, want 0/0", storeCalls, nowCalls)
			}
		})
	}
}

func TestResolveUsesOneTransactionAndTimestampForTheCascade(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		exit           Exit
		value          string
		cancelledTasks []task.Task
	}{
		{name: "done", exit: ExitDone, value: "done", cancelledTasks: []task.Task{{ID: 12}}},
		{name: "cancelled", exit: ExitCancelled, value: "cancelled"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resolvedProject := Project{ID: 7, Status: test.value, Tags: domain.TagNames{}}
			transactionStore := &recordingStore{
				resolveResult: resolvedProject,
				cancelResult:  test.cancelledTasks,
			}
			store := &recordingStore{transactionStore: transactionStore}
			service := NewService(store)
			nowCalls := 0
			service.now = func() time.Time {
				nowCalls++
				return time.Date(
					2026,
					time.July,
					27,
					12,
					34,
					56,
					987654321,
					time.FixedZone("offset", -4*60*60),
				)
			}

			resolution, err := service.Resolve(context.Background(), 7, test.exit)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if string(test.exit) != test.value {
				t.Errorf("exit value = %q, want %q", test.exit, test.value)
			}
			if store.transactionCalls != 1 || store.resolveCalls != 0 || store.cancelOpenTasksCalls != 0 {
				t.Errorf(
					"outer transaction/resolve/cancel calls = %d/%d/%d, want 1/0/0",
					store.transactionCalls,
					store.resolveCalls,
					store.cancelOpenTasksCalls,
				)
			}
			if transactionStore.resolveCalls != 1 || transactionStore.cancelOpenTasksCalls != 1 {
				t.Errorf("transaction-scoped calls = %#v, want one resolve and one task cancellation", transactionStore)
			}
			const wantTimestamp = "2026-07-27T16:34:56.987Z"
			if nowCalls != 1 || transactionStore.resolveTimestamp != wantTimestamp ||
				transactionStore.cancelTimestamp != wantTimestamp {
				t.Errorf(
					"clock/resolve/cancel timestamps = %d/%q/%q, want 1/%q/%q",
					nowCalls,
					transactionStore.resolveTimestamp,
					transactionStore.cancelTimestamp,
					wantTimestamp,
					wantTimestamp,
				)
			}
			if transactionStore.resolveID != 7 || transactionStore.cancelProjectID != 7 ||
				transactionStore.resolveExit != test.exit {
				t.Errorf(
					"resolve/cancel IDs and exit = %d/%d/%q, want 7/7/%q",
					transactionStore.resolveID,
					transactionStore.cancelProjectID,
					transactionStore.resolveExit,
					test.exit,
				)
			}
			if !reflect.DeepEqual(resolution.Project, resolvedProject) {
				t.Errorf("resolution project = %#v, want %#v", resolution.Project, resolvedProject)
			}
			if resolution.CancelledTasks == nil ||
				len(resolution.CancelledTasks) != len(test.cancelledTasks) {
				t.Errorf(
					"cancelled tasks = %#v, want non-nil list of length %d",
					resolution.CancelledTasks,
					len(test.cancelledTasks),
				)
			}
		})
	}
}

func TestReopenDelegatesOneTimestampWithoutATransaction(t *testing.T) {
	t.Parallel()

	store := &recordingStore{reopenResult: Project{ID: 7, Status: string(ListStatusOpen), Tags: domain.TagNames{}}}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.UTC)
	}

	reopened, err := service.Reopen(context.Background(), 7)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if !reflect.DeepEqual(reopened, store.reopenResult) || store.reopenCalls != 1 || store.reopenID != 7 {
		t.Errorf(
			"Reopen() result/calls/ID = %#v/%d/%d, want %#v/1/7",
			reopened,
			store.reopenCalls,
			store.reopenID,
			store.reopenResult,
		)
	}
	if nowCalls != 1 || store.reopenTimestamp != "2026-07-27T12:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want one UTC timestamp", nowCalls, store.reopenTimestamp)
	}
	if store.transactionCalls != 0 {
		t.Errorf("transaction calls = %d, want 0", store.transactionCalls)
	}
}

func TestTagAndUntagNormalizeAndRefreshWithinOneTransaction(t *testing.T) {
	t.Parallel()

	resolvedTags := []tag.Tag{{ID: 10, Title: "WORK"}, {ID: 11, Title: "É"}, {ID: 12, Title: "é"}}
	wantNames := []string{"Work", "É", "é"}
	refreshed := Project{ID: 7, Title: "project", Tags: domain.TagNames{"stored"}}
	tests := []struct {
		name  string
		untag bool
		apply func(*Service) (Tagging, error)
	}{
		{
			name: "tag",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"Work", "work", "É", "é"})
			},
		},
		{
			name:  "untag",
			untag: true,
			apply: func(service *Service) (Tagging, error) {
				return service.Untag(context.Background(), 7, []string{"Work", "work", "É", "é"})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactionStore := &recordingStore{
				findResults:       []Project{{ID: 7, Title: "before"}, refreshed},
				resolveTagsResult: resolvedTags,
			}
			store := &recordingStore{transactionStore: transactionStore}
			tagging, err := test.apply(NewService(store))
			if err != nil {
				t.Fatalf("tag mutation error = %v", err)
			}
			if store.transactionCalls != 1 || store.findCalls != 0 || store.resolveTagsCalls != 0 ||
				store.attachTagsCalls != 0 || store.detachTagsCalls != 0 {
				t.Errorf(
					"outer transaction/find/resolve/attach/detach calls = %d/%d/%d/%d/%d, want 1/0/0/0/0",
					store.transactionCalls,
					store.findCalls,
					store.resolveTagsCalls,
					store.attachTagsCalls,
					store.detachTagsCalls,
				)
			}
			if transactionStore.findCalls != 2 || transactionStore.findID != 7 ||
				transactionStore.resolveTagsCalls != 1 || !slices.Equal(transactionStore.resolveTagNames, wantNames) {
				t.Errorf(
					"transaction find calls/ID/names = %d/%d/%v, want 2/7/%v",
					transactionStore.findCalls,
					transactionStore.findID,
					transactionStore.resolveTagNames,
					wantNames,
				)
			}
			if transactionStore.attachTagsCalls+transactionStore.detachTagsCalls != 1 ||
				transactionStore.attachProjectID+transactionStore.detachProjectID != 7 {
				t.Errorf(
					"transaction attach/detach calls and IDs = %d/%d/%d/%d, want one mutation for project 7",
					transactionStore.attachTagsCalls,
					transactionStore.detachTagsCalls,
					transactionStore.attachProjectID,
					transactionStore.detachProjectID,
				)
			}
			mutatedTags := transactionStore.attachedTags
			if test.untag {
				mutatedTags = transactionStore.detachedTags
			}
			if !slices.Equal(mutatedTags, resolvedTags) {
				t.Errorf("mutated tags = %#v, want resolved tags %#v", mutatedTags, resolvedTags)
			}
			if !reflect.DeepEqual(tagging.Project, refreshed) ||
				!slices.Equal(tagging.TagTitles, []string{"WORK", "É", "é"}) {
				t.Errorf("tag mutation result = %#v, want refreshed project and stored tag titles", tagging)
			}
		})
	}
}

func TestTagAndUntagRejectInvalidInputBeforeStartingATransaction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{
			name: "invalid project ID",
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 0, []string{"work"})
				return err
			},
		},
		{
			name: "no tags",
			apply: func(service *Service) error {
				_, err := service.Untag(context.Background(), 7, nil)
				return err
			},
		},
		{
			name: "blank tag",
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work", "\t"})
				return err
			},
		},
		{
			name: "invalid UTF-8 tag",
			apply: func(service *Service) error {
				_, err := service.Untag(context.Background(), 7, []string{string([]byte{0xff})})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			if err := test.apply(NewService(store)); errorCode(err) != apperr.InvalidArgument {
				t.Errorf("tag mutation error = %v, want invalid_argument", err)
			}
			if store.transactionCalls != 0 || store.findCalls != 0 || store.resolveTagsCalls != 0 ||
				store.attachTagsCalls != 0 || store.detachTagsCalls != 0 {
				t.Errorf("store calls occurred before validation: %#v", store)
			}
		})
	}
}

func TestTaggedProjectStoreErrorsPassThroughUnchanged(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	tests := []struct {
		name      string
		configure func(*recordingStore, *recordingStore)
		apply     func(*Service) error
	}{
		{
			name: "add project",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.addError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Add(context.Background(), AddFields{Title: "project", Tags: []string{"work"}})
				return err
			},
		},
		{
			name: "add resolve tags",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.resolveTagsError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Add(context.Background(), AddFields{Title: "project", Tags: []string{"work"}})
				return err
			},
		},
		{
			name: "add attach tags",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.attachTagsError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Add(context.Background(), AddFields{Title: "project", Tags: []string{"work"}})
				return err
			},
		},
		{
			name: "add refresh",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.findErrors = []error{storeError}
			},
			apply: func(service *Service) error {
				_, err := service.Add(context.Background(), AddFields{Title: "project", Tags: []string{"work"}})
				return err
			},
		},
		{
			name: "tag initial find",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.findErrors = []error{storeError}
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work"})
				return err
			},
		},
		{
			name: "tag resolve",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.resolveTagsError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work"})
				return err
			},
		},
		{
			name: "tag attach",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.attachTagsError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work"})
				return err
			},
		},
		{
			name: "untag detach",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.detachTagsError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Untag(context.Background(), 7, []string{"work"})
				return err
			},
		},
		{
			name: "tag refresh",
			configure: func(_ *recordingStore, transactionStore *recordingStore) {
				transactionStore.findErrors = []error{nil, storeError}
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work"})
				return err
			},
		},
		{
			name: "transaction",
			configure: func(store, _ *recordingStore) {
				store.transactionError = storeError
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"work"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactionStore := &recordingStore{resolveTagsResult: []tag.Tag{{ID: 1, Title: "work"}}}
			store := &recordingStore{transactionStore: transactionStore}
			test.configure(store, transactionStore)
			if err := test.apply(NewService(store)); !errors.Is(err, storeError) {
				t.Errorf("tagged project error = %v, want preserved store error %v", err, storeError)
			}
		})
	}
}

func TestDeleteUsesATransactionOnlyForRecursiveDeletion(t *testing.T) {
	t.Parallel()

	t.Run("nonrecursive", func(t *testing.T) {
		t.Parallel()

		deletedProject := Project{ID: 7, Title: "empty", Tags: domain.TagNames{}}
		store := &recordingStore{deleteResult: deletedProject}
		deletion, err := NewService(store).Delete(context.Background(), 7, false)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if store.transactionCalls != 0 || store.deleteCalls != 1 || store.deleteTasksCalls != 0 {
			t.Errorf(
				"transaction/project/task delete calls = %d/%d/%d, want 0/1/0",
				store.transactionCalls,
				store.deleteCalls,
				store.deleteTasksCalls,
			)
		}
		if !reflect.DeepEqual(deletion.Project, deletedProject) || deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
			t.Errorf("Delete() = %#v, want project and non-nil empty deleted tasks", deletion)
		}
	})

	t.Run("recursive", func(t *testing.T) {
		t.Parallel()

		deletedProject := Project{ID: 7, Title: "populated", Tags: domain.TagNames{}}
		deletedTasks := []task.Task{{ID: 8}, {ID: 9}}
		transactionStore := &recordingStore{
			deleteResult:      deletedProject,
			deleteTasksResult: deletedTasks,
		}
		store := &recordingStore{transactionStore: transactionStore}
		deletion, err := NewService(store).Delete(context.Background(), 7, true)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if store.transactionCalls != 1 || store.deleteCalls != 0 || store.deleteTasksCalls != 0 {
			t.Errorf(
				"outer transaction/project/task delete calls = %d/%d/%d, want 1/0/0",
				store.transactionCalls,
				store.deleteCalls,
				store.deleteTasksCalls,
			)
		}
		if transactionStore.deleteTasksCalls != 1 || transactionStore.deleteCalls != 1 {
			t.Errorf("transaction-scoped calls = %#v, want one task deletion and one project deletion", transactionStore)
		}
		if transactionStore.deleteTasksProjectID != 7 || transactionStore.deleteID != 7 {
			t.Errorf(
				"task/project delete IDs = %d/%d, want 7/7",
				transactionStore.deleteTasksProjectID,
				transactionStore.deleteID,
			)
		}
		if !reflect.DeepEqual(deletion.Project, deletedProject) || len(deletion.DeletedTasks) != len(deletedTasks) ||
			deletion.DeletedTasks[0].ID != 8 || deletion.DeletedTasks[1].ID != 9 {
			t.Errorf("Delete() = %#v, want project and deleted tasks", deletion)
		}
	})

	t.Run("recursive empty project", func(t *testing.T) {
		t.Parallel()

		transactionStore := &recordingStore{}
		store := &recordingStore{transactionStore: transactionStore}
		deletion, err := NewService(store).Delete(context.Background(), 7, true)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
			t.Errorf("deleted tasks = %#v, want non-nil empty list", deletion.DeletedTasks)
		}
	})
}

func TestLifecycleStoreErrorsPassThroughUnchanged(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	tests := []struct {
		name  string
		store *recordingStore
		apply func(*Service) error
	}{
		{
			name:  "resolve project",
			store: &recordingStore{resolveError: storeError},
			apply: func(service *Service) error {
				_, err := service.Resolve(context.Background(), 7, ExitDone)
				return err
			},
		},
		{
			name:  "cancel open tasks",
			store: &recordingStore{cancelError: storeError},
			apply: func(service *Service) error {
				_, err := service.Resolve(context.Background(), 7, ExitDone)
				return err
			},
		},
		{
			name:  "reopen",
			store: &recordingStore{reopenError: storeError},
			apply: func(service *Service) error {
				_, err := service.Reopen(context.Background(), 7)
				return err
			},
		},
		{
			name:  "nonrecursive delete",
			store: &recordingStore{deleteError: storeError},
			apply: func(service *Service) error {
				_, err := service.Delete(context.Background(), 7, false)
				return err
			},
		},
		{
			name:  "recursive task delete",
			store: &recordingStore{deleteTasksError: storeError},
			apply: func(service *Service) error {
				_, err := service.Delete(context.Background(), 7, true)
				return err
			},
		},
		{
			name:  "recursive project delete",
			store: &recordingStore{deleteError: storeError},
			apply: func(service *Service) error {
				_, err := service.Delete(context.Background(), 7, true)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.apply(NewService(test.store)); !errors.Is(err, storeError) {
				t.Errorf("lifecycle error = %v, want preserved store error %v", err, storeError)
			}
		})
	}
}

func equalCreateFields(left CreateFields, right AddFields) bool {
	return left.AreaID == right.AreaID && left.StageID == nil && left.Title == right.Title && left.Note == right.Note
}

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
