package task

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
)

type recordingStore struct {
	addCalls               int
	title                  string
	note                   string
	addedProjectID         *int64
	addedAreaID            *int64
	addedDueOn             *string
	addedDeferUntil        *string
	addedDeferStageID      *int64
	addedPromotes          bool
	addedTags              []string
	timestamp              string
	addResult              *Task
	addError               error
	inboxCalls             int
	inboxResult            []ViewTask
	availableCalls         int
	availableResult        []ViewTask
	findCalls              int
	findResults            []Task
	findError              error
	findErrors             []error
	listCalls              int
	listedFilter           ListFilter
	listResult             []Task
	listError              error
	projectExistsCalls     int
	projectExistsError     error
	findProjectCalls       int
	findProjectID          int64
	findProjectResult      domain.Project
	findProjectError       error
	findStageCalls         int
	findStageProjectID     int64
	findStageTitle         string
	findStageResult        StageReference
	findStageError         error
	findStageByIDCalls     int
	findStageByIDID        int64
	findStageByIDResult    StageReference
	findStageByIDError     error
	stageExistsCalls       int
	stageExistsTitle       string
	stageExistsResult      bool
	stageExistsError       error
	findNextStageCalls     int
	findNextStageProjectID int64
	findNextStageCurrentID int64
	findNextStageResult    *StageReference
	findNextStageError     error
	moveProjectStageCalls  int
	moveProjectID          int64
	moveProjectStageID     int64
	moveProjectTimestamp   string
	moveProjectResult      domain.Project
	moveProjectError       error
	areaExistsCalls        int
	areaExistsError        error
	editCalls              int
	editID                 int64
	editFields             EditFields
	editTimestamp          string
	editResult             *Task
	editError              error
	reorderCalls           int
	reorderID              int64
	reorderPlacement       domain.Placement
	reorderTimestamp       string
	reorderResult          Task
	reorderError           error
	doneCalls              int
	doneResult             *Task
	doneError              error
	cancelCalls            int
	reopenCalls            int
	deleteCalls            int
	lifecycleID            int64
	lifecycleTimestamp     string
	resolveCalls           int
	resolvedNames          []string
	resolveResult          []tag.Tag
	resolveError           error
	attachCalls            int
	attachedID             int64
	attachedTags           []tag.Tag
	attachError            error
	detachCalls            int
	detachedID             int64
	detachedTags           []tag.Tag
	detachError            error
	transactionCalls       int
	transactionStore       Transaction
	transactionError       error
	readTransactionCalls   int
	readTransactionStore   Transaction
	readTransactionError   error
}

func (r *recordingStore) Add(
	_ context.Context,
	fields AddFields,
	timestamp string,
) (Task, error) {
	r.addCalls++
	r.title = fields.Title
	r.note = fields.Note
	r.addedProjectID = fields.ProjectID
	r.addedAreaID = fields.AreaID
	r.addedDueOn = fields.DueOn
	r.addedDeferUntil = fields.DeferUntil
	r.addedDeferStageID = fields.DeferStageID
	r.addedPromotes = fields.Promotes
	r.addedTags = fields.Tags
	r.timestamp = timestamp
	if r.addError != nil {
		return Task{}, r.addError
	}
	if r.addResult != nil {
		return *r.addResult, nil
	}

	return Task{
		ID:           1,
		ProjectID:    fields.ProjectID,
		AreaID:       fields.AreaID,
		Title:        fields.Title,
		Note:         fields.Note,
		DueOn:        fields.DueOn,
		DeferUntil:   fields.DeferUntil,
		DeferStageID: fields.DeferStageID,
		Promotes:     fields.Promotes,
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}, nil
}

func (r *recordingStore) Inbox(context.Context) ([]ViewTask, error) {
	r.inboxCalls++
	return r.inboxResult, nil
}

func (r *recordingStore) Available(context.Context) ([]ViewTask, error) {
	r.availableCalls++
	return r.availableResult, nil
}

func (r *recordingStore) Find(_ context.Context, id int64) (Task, error) {
	r.findCalls++
	if r.findError != nil {
		return Task{}, r.findError
	}
	if r.findCalls <= len(r.findErrors) && r.findErrors[r.findCalls-1] != nil {
		return Task{}, r.findErrors[r.findCalls-1]
	}
	if r.findCalls <= len(r.findResults) {
		return r.findResults[r.findCalls-1], nil
	}
	return Task{ID: id}, nil
}

func (r *recordingStore) List(_ context.Context, filter ListFilter) ([]Task, error) {
	r.listCalls++
	r.listedFilter = filter
	return r.listResult, r.listError
}

func (r *recordingStore) ProjectExists(context.Context, int64) error {
	r.projectExistsCalls++
	return r.projectExistsError
}

func (r *recordingStore) FindProject(_ context.Context, id int64) (domain.Project, error) {
	r.findProjectCalls++
	r.findProjectID = id
	return r.findProjectResult, r.findProjectError
}

func (r *recordingStore) FindStage(_ context.Context, projectID int64, title string) (StageReference, error) {
	r.findStageCalls++
	r.findStageProjectID = projectID
	r.findStageTitle = title
	return r.findStageResult, r.findStageError
}

func (r *recordingStore) FindStageByID(_ context.Context, id int64) (StageReference, error) {
	r.findStageByIDCalls++
	r.findStageByIDID = id
	return r.findStageByIDResult, r.findStageByIDError
}

func (r *recordingStore) StageExists(_ context.Context, title string) (bool, error) {
	r.stageExistsCalls++
	r.stageExistsTitle = title
	return r.stageExistsResult, r.stageExistsError
}

func (r *recordingStore) FindNextStage(_ context.Context, projectID, currentStageID int64) (*StageReference, error) {
	r.findNextStageCalls++
	r.findNextStageProjectID = projectID
	r.findNextStageCurrentID = currentStageID
	return r.findNextStageResult, r.findNextStageError
}

func (r *recordingStore) MoveProjectStage(
	_ context.Context,
	projectID, stageID int64,
	timestamp string,
) (domain.Project, error) {
	r.moveProjectStageCalls++
	r.moveProjectID = projectID
	r.moveProjectStageID = stageID
	r.moveProjectTimestamp = timestamp
	return r.moveProjectResult, r.moveProjectError
}

func (r *recordingStore) AreaExists(context.Context, int64) error {
	r.areaExistsCalls++
	return r.areaExistsError
}

func (r *recordingStore) Edit(
	_ context.Context,
	id int64,
	fields EditFields,
	timestamp string,
) (Task, error) {
	r.editCalls++
	r.editID = id
	r.editFields = fields
	r.editTimestamp = timestamp
	if r.editError != nil {
		return Task{}, r.editError
	}
	if r.editResult != nil {
		return *r.editResult, nil
	}

	return Task{ID: id, UpdatedAt: timestamp}, nil
}

func (r *recordingStore) Reorder(
	_ context.Context,
	id int64,
	placement domain.Placement,
	timestamp string,
) (Task, error) {
	r.reorderCalls++
	r.reorderID = id
	r.reorderPlacement = placement
	r.reorderTimestamp = timestamp
	return r.reorderResult, r.reorderError
}

func (r *recordingStore) Done(_ context.Context, id int64, timestamp string) (Task, error) {
	r.doneCalls++
	r.recordLifecycle(id, timestamp)
	if r.doneError != nil {
		return Task{}, r.doneError
	}
	if r.doneResult != nil {
		return *r.doneResult, nil
	}
	return Task{ID: id, DoneAt: &timestamp}, nil
}

func (r *recordingStore) Cancel(_ context.Context, id int64, timestamp string) (Task, error) {
	r.cancelCalls++
	r.recordLifecycle(id, timestamp)
	return Task{ID: id, CancelledAt: &timestamp}, nil
}

func (r *recordingStore) Reopen(_ context.Context, id int64, timestamp string) (Task, error) {
	r.reopenCalls++
	r.recordLifecycle(id, timestamp)
	return Task{ID: id}, nil
}

func (r *recordingStore) Delete(_ context.Context, id int64) (Task, error) {
	r.deleteCalls++
	r.lifecycleID = id
	return Task{ID: id}, nil
}

func (r *recordingStore) ResolveTags(_ context.Context, names []string) ([]tag.Tag, error) {
	r.resolveCalls++
	r.resolvedNames = names
	return r.resolveResult, r.resolveError
}

func (r *recordingStore) AttachTags(_ context.Context, id int64, tags []tag.Tag) error {
	r.attachCalls++
	r.attachedID = id
	r.attachedTags = tags
	return r.attachError
}

func (r *recordingStore) DetachTags(_ context.Context, id int64, tags []tag.Tag) error {
	r.detachCalls++
	r.detachedID = id
	r.detachedTags = tags
	return r.detachError
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

func (r *recordingStore) recordLifecycle(id int64, timestamp string) {
	r.lifecycleID = id
	r.lifecycleTimestamp = timestamp
}

func TestAddPreservesAcceptedTextAndNormalizesTimestamp(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	title := "  Keep surrounding space  "
	note := "line one\nline two\n"
	projectID := int64(7)
	dueOn := "tomorrow"
	deferUntil := "today"
	created, err := service.Add(context.Background(), AddRequest{
		ProjectID:  &projectID,
		Title:      title,
		Note:       note,
		DueOn:      &dueOn,
		DeferUntil: &deferUntil,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.title != title || created.Title != title {
		t.Errorf("title = %q, want exact %q", created.Title, title)
	}
	if store.note != note || created.Note != note {
		t.Errorf("note = %q, want exact %q", created.Note, note)
	}
	if store.addedProjectID == nil || *store.addedProjectID != projectID ||
		created.ProjectID == nil || *created.ProjectID != projectID {
		t.Errorf(
			"project ID = %#v/%#v, want %d",
			store.addedProjectID,
			created.ProjectID,
			projectID,
		)
	}
	if store.addedDueOn == nil || *store.addedDueOn != "2026-07-28" ||
		created.DueOn == nil || *created.DueOn != "2026-07-28" {
		t.Errorf("due date = %#v/%#v, want canonical 2026-07-28", store.addedDueOn, created.DueOn)
	}
	if store.addedDeferUntil == nil || *store.addedDeferUntil != "2026-07-27" ||
		created.DeferUntil == nil || *created.DeferUntil != "2026-07-27" {
		t.Errorf("defer date = %#v/%#v, want canonical 2026-07-27", store.addedDeferUntil, created.DeferUntil)
	}
	if nowCalls != 1 || store.timestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want one call and UTC milliseconds", nowCalls, store.timestamp)
	}
}

func TestAddValidatesAndDelegatesArea(t *testing.T) {
	t.Parallel()

	areaID := int64(11)
	store := &recordingStore{}
	if _, err := NewService(store).Add(context.Background(), AddRequest{AreaID: &areaID, Title: "area task"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.addCalls != 1 || store.addedAreaID == nil || *store.addedAreaID != areaID {
		t.Errorf("store Add() calls/area = %d/%#v, want 1/%d", store.addCalls, store.addedAreaID, areaID)
	}

	projectID := int64(7)
	invalidAreaID := int64(0)
	for _, fields := range []AddRequest{
		{AreaID: &invalidAreaID, Title: "valid"},
		{ProjectID: &projectID, AreaID: &areaID, Title: "valid"},
	} {
		store := &recordingStore{}
		_, err := NewService(store).Add(context.Background(), fields)
		if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
			t.Errorf("Add(%#v) error = %v, want invalid_argument", fields, err)
		}
		if store.addCalls != 0 {
			t.Errorf("store Add() calls = %d, want 0", store.addCalls)
		}
	}
}

func TestAddRejectsInvalidTextBeforePersistence(t *testing.T) {
	t.Parallel()

	invalidDate := "2026-02-30"
	blankDeferStage := " "
	invalidProjectID := int64(0)
	tests := []struct {
		name       string
		projectID  *int64
		title      string
		note       string
		dueOn      *string
		deferUntil *string
		deferStage *string
		tags       []string
	}{
		{name: "blank title", title: " \t\n"},
		{name: "invalid title UTF-8", title: string([]byte{0xff})},
		{name: "invalid note UTF-8", title: "valid", note: string([]byte{0xff})},
		{name: "nonpositive project ID", projectID: &invalidProjectID, title: "valid"},
		{name: "invalid due date", title: "valid", dueOn: &invalidDate},
		{name: "invalid defer date", title: "valid", deferUntil: &invalidDate},
		{name: "blank defer stage", title: "valid", deferStage: &blankDeferStage},
		{name: "blank tag", title: "valid", tags: []string{" "}},
		{name: "invalid tag UTF-8", title: "valid", tags: []string{string([]byte{0xff})}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			service := NewService(store)
			_, err := service.Add(context.Background(), AddRequest{
				ProjectID:  test.projectID,
				Title:      test.title,
				Note:       test.note,
				DueOn:      test.dueOn,
				DeferUntil: test.deferUntil,
				DeferStage: test.deferStage,
				Tags:       test.tags,
			})
			if err == nil {
				t.Fatal("Add() error = nil, want invalid_argument")
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("Add() error = %v, want invalid_argument", err)
			}
			if store.addCalls != 0 || store.transactionCalls != 0 {
				t.Errorf("store Add()/transaction calls = %d/%d, want 0/0", store.addCalls, store.transactionCalls)
			}
			rejected := test.dueOn
			if rejected == nil {
				rejected = test.deferUntil
			}
			if rejected != nil && !strings.Contains(err.Error(), *rejected) {
				t.Errorf("Add() error = %q, want rejected input", err)
			}
		})
	}
}

func TestAddWithTagsOwnsTransactionNormalizesNamesAndReturnsRefresh(t *testing.T) {
	t.Parallel()

	created := Task{ID: 7, Title: "tagged task"}
	refreshed := Task{ID: 7, Title: "tagged task", Tags: []string{"Stored", "é", "É"}}
	resolved := []tag.Tag{
		{ID: 1, Title: "Stored"},
		{ID: 2, Title: "é"},
		{ID: 3, Title: "É"},
	}
	transactionStore := &recordingStore{
		addResult:     &created,
		resolveResult: resolved,
		findResults:   []Task{refreshed},
	}
	store := &recordingStore{transactionStore: transactionStore}

	got, err := NewService(store).Add(context.Background(), AddRequest{
		Title: "tagged task",
		Tags:  []string{"Errands", "ERRANDS", "é", "É"},
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !reflect.DeepEqual(got, refreshed) {
		t.Errorf("Add() = %#v, want refreshed %#v", got, refreshed)
	}
	if store.transactionCalls != 1 || store.addCalls+store.resolveCalls+store.attachCalls+store.findCalls != 0 {
		t.Errorf("outer store calls = %#v, want transaction boundary only", store)
	}
	wantNames := []string{"Errands", "é", "É"}
	if !reflect.DeepEqual(transactionStore.addedTags, wantNames) ||
		!reflect.DeepEqual(transactionStore.resolvedNames, wantNames) {
		t.Errorf(
			"normalized add/resolve names = %v/%v, want %v",
			transactionStore.addedTags,
			transactionStore.resolvedNames,
			wantNames,
		)
	}
	if transactionStore.addCalls != 1 || transactionStore.resolveCalls != 1 ||
		transactionStore.attachCalls != 1 || transactionStore.findCalls != 1 {
		t.Errorf("transaction-scoped calls = %#v, want one add, resolve, attach, and refresh", transactionStore)
	}
	if transactionStore.attachedID != created.ID || !reflect.DeepEqual(transactionStore.attachedTags, resolved) {
		t.Errorf(
			"attached ID/tags = %d/%#v, want %d/%#v",
			transactionStore.attachedID,
			transactionStore.attachedTags,
			created.ID,
			resolved,
		)
	}
}

func TestAddWithoutTagsKeepsDirectStorePath(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	if _, err := NewService(store).Add(context.Background(), AddRequest{Title: "untagged"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.addCalls != 1 || store.transactionCalls != 0 {
		t.Errorf("add/transaction calls = %d/%d, want 1/0", store.addCalls, store.transactionCalls)
	}
}

func TestAddWithTagsReturnsTransactionErrors(t *testing.T) {
	t.Parallel()

	storeError := errors.New("store failure")
	tests := []struct {
		name        string
		transaction *recordingStore
		outerError  error
	}{
		{name: "transaction boundary", transaction: &recordingStore{}, outerError: storeError},
		{name: "add", transaction: &recordingStore{addError: storeError}},
		{name: "resolve unknown", transaction: &recordingStore{resolveError: storeError}},
		{
			name:        "attach",
			transaction: &recordingStore{resolveResult: []tag.Tag{{ID: 1, Title: "known"}}, attachError: storeError},
		},
		{
			name:        "refresh",
			transaction: &recordingStore{resolveResult: []tag.Tag{{ID: 1, Title: "known"}}, findError: storeError},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{
				transactionStore: test.transaction,
				transactionError: test.outerError,
			}
			_, err := NewService(store).Add(context.Background(), AddRequest{
				Title: "tagged",
				Tags:  []string{"known"},
			})
			if !errors.Is(err, storeError) {
				t.Fatalf("Add() error = %v, want store failure", err)
			}
			if store.transactionCalls != 1 || store.addCalls+store.resolveCalls+store.attachCalls+store.findCalls != 0 {
				t.Errorf("outer store calls = %#v, want transaction boundary only", store)
			}
		})
	}
}

func TestAddResolvesDeferStageOnTheProjectBoard(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	projectStageID := int64(11)
	target := StageReference{ID: 13, BoardID: 17, Title: "Waiting", Position: 3}
	notFound := apperr.New(apperr.NotFound, "stage Waiting not found", nil)
	tests := []struct {
		name     string
		request  AddRequest
		store    *recordingStore
		wantCode apperr.Code
	}{
		{
			name:     "project required",
			request:  AddRequest{Title: "task", DeferStage: &target.Title},
			store:    &recordingStore{},
			wantCode: apperr.InvalidArgument,
		},
		{
			name:    "project must be on board",
			request: AddRequest{ProjectID: &projectID, Title: "task", DeferStage: &target.Title},
			store: &recordingStore{
				findProjectResult: domain.Project{ID: projectID},
			},
			wantCode: apperr.InvalidArgument,
		},
		{
			name:    "unknown title remains not found",
			request: AddRequest{ProjectID: &projectID, Title: "task", DeferStage: &target.Title},
			store: &recordingStore{
				findProjectResult:   domain.Project{ID: projectID, StageID: &projectStageID},
				findStageByIDResult: StageReference{ID: projectStageID, BoardID: target.BoardID},
				findStageError:      notFound,
			},
			wantCode: apperr.NotFound,
		},
		{
			name:    "title only on another board is invalid",
			request: AddRequest{ProjectID: &projectID, Title: "task", DeferStage: &target.Title},
			store: &recordingStore{
				findProjectResult:   domain.Project{ID: projectID, StageID: &projectStageID},
				findStageByIDResult: StageReference{ID: projectStageID, BoardID: target.BoardID},
				findStageError:      notFound,
				stageExistsResult:   true,
			},
			wantCode: apperr.InvalidArgument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outer := &recordingStore{transactionStore: test.store}
			_, err := NewService(outer).Add(context.Background(), test.request)
			if code, ok := apperr.CodeOf(err); !ok || code != test.wantCode {
				t.Fatalf("Add() error = %v, want %s", err, test.wantCode)
			}
			if outer.transactionCalls != 1 || outer.addCalls != 0 || test.store.addCalls != 0 {
				t.Errorf("transaction/outer add/transaction add calls = %d/%d/%d, want 1/0/0", outer.transactionCalls, outer.addCalls, test.store.addCalls)
			}
		})
	}

	transaction := &recordingStore{
		findProjectResult:   domain.Project{ID: projectID, StageID: &projectStageID},
		findStageByIDResult: StageReference{ID: projectStageID, BoardID: target.BoardID},
		findStageResult:     target,
	}
	outer := &recordingStore{transactionStore: transaction}
	created, err := NewService(outer).Add(context.Background(), AddRequest{
		ProjectID:  &projectID,
		Title:      "task",
		DeferStage: &target.Title,
		Promotes:   true,
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if outer.transactionCalls != 1 || transaction.addCalls != 1 ||
		transaction.findProjectID != projectID || transaction.findStageByIDCalls != 1 ||
		transaction.findStageByIDID != projectStageID ||
		transaction.findStageProjectID != target.BoardID || transaction.findStageTitle != target.Title {
		t.Errorf("defer-stage resolution calls = %#v, want one transaction and project-board lookup", transaction)
	}
	if transaction.addedDeferStageID == nil || *transaction.addedDeferStageID != target.ID ||
		created.DeferStageID == nil || *created.DeferStageID != target.ID {
		t.Errorf("persisted/result defer stage = %#v/%#v, want %d", transaction.addedDeferStageID, created.DeferStageID, target.ID)
	}
	if !transaction.addedPromotes || !created.Promotes {
		t.Errorf("persisted/result promotes = %t/%t, want true", transaction.addedPromotes, created.Promotes)
	}
}

func TestParseID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  int64
		valid bool
	}{
		{value: "1", want: 1, valid: true},
		{value: "001", want: 1, valid: true},
		{value: "0"},
		{value: "-1"},
		{value: "+1"},
		{value: "1.0"},
		{value: "１２"},
		{value: "9223372036854775808"},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := ParseID(test.value)
			if test.valid {
				if err != nil {
					t.Fatalf("ParseID() error = %v", err)
				}
				if got != test.want {
					t.Errorf("ParseID() = %d, want %d", got, test.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("ParseID() = %d, want invalid_argument", got)
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("ParseID() error = %v, want invalid_argument", err)
			}
		})
	}
}

func TestShowRejectsNonpositiveIDBeforePersistence(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	_, err := service.Show(context.Background(), 0)
	if err == nil {
		t.Fatal("Show() error = nil, want invalid_argument")
	}
	if store.findCalls != 0 {
		t.Errorf("store Find() calls = %d, want 0", store.findCalls)
	}
}

func TestParseListStatus(t *testing.T) {
	t.Parallel()

	for _, value := range []ListStatus{
		ListStatusOpen,
		ListStatusDone,
		ListStatusCancelled,
		ListStatusAll,
	} {
		value := value
		t.Run(string(value), func(t *testing.T) {
			t.Parallel()
			got, err := ParseListStatus(string(value))
			if err != nil {
				t.Fatalf("ParseListStatus() error = %v", err)
			}
			if got != value {
				t.Errorf("ParseListStatus() = %q, want %q", got, value)
			}
		})
	}

	if _, err := ParseListStatus("OPEN"); err == nil {
		t.Fatal("ParseListStatus(OPEN) error = nil, want invalid_argument")
	} else if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
		t.Errorf("ParseListStatus(OPEN) error = %v, want invalid_argument", err)
	}
}

func TestInboxNormalizesNil(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	inbox, err := NewService(store).Inbox(context.Background())
	if err != nil {
		t.Fatalf("Inbox() error = %v", err)
	}
	if inbox == nil || len(inbox) != 0 {
		t.Errorf("Inbox() = %#v, want non-nil empty list", inbox)
	}
	if store.inboxCalls != 1 {
		t.Errorf("store Inbox() calls = %d, want 1", store.inboxCalls)
	}
}

func TestAvailableNormalizesNil(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	available, err := NewService(store).Available(context.Background())
	if err != nil {
		t.Fatalf("Available() error = %v", err)
	}
	if available == nil || len(available) != 0 {
		t.Errorf("Available() = %#v, want non-nil empty list", available)
	}
	if store.availableCalls != 1 {
		t.Errorf("store Available() calls = %d, want 1", store.availableCalls)
	}
}

func TestListValidatesOptionsAndNormalizesNil(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	options := ListOptions{
		Status: ListStatusDone,
		Date:   DateSelectorDeferred,
	}

	listed, err := service.List(context.Background(), options)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("List() = %#v, want non-nil empty list", listed)
	}
	wantFilter := ListFilter{Status: options.Status, Date: options.Date}
	if store.listCalls != 1 || store.listedFilter != wantFilter || store.readTransactionCalls != 0 {
		t.Errorf("store List() calls/filter/read transactions = %d/%#v/%d, want 1/%#v/0", store.listCalls, store.listedFilter, store.readTransactionCalls, wantFilter)
	}

	invalidProjectID := int64(0)
	invalid := []ListOptions{
		{Status: ListStatus("invalid")},
		{Status: ListStatusOpen, Date: DateSelector("invalid")},
		{Status: ListStatusOpen, ProjectID: &invalidProjectID},
	}
	for _, request := range invalid {
		_, err = service.List(context.Background(), request)
		if err == nil {
			t.Fatalf("List(%#v) error = nil, want invalid_argument", request)
		}
		if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
			t.Errorf("List(%#v) error = %v, want invalid_argument", request, err)
		}
	}
	if store.listCalls != 1 {
		t.Errorf("store List() calls = %d, want 1", store.listCalls)
	}
}

func TestListValidatesAndDelegatesArea(t *testing.T) {
	t.Parallel()

	areaID := int64(11)
	store := &recordingStore{}
	options := ListOptions{Status: ListStatusOpen, AreaID: &areaID}
	if _, err := NewService(store).List(context.Background(), options); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantFilter := ListFilter{Status: options.Status, AreaID: options.AreaID}
	if store.readTransactionCalls != 1 || store.areaExistsCalls != 1 ||
		store.listCalls != 1 || store.listedFilter != wantFilter {
		t.Errorf("store read/area/list calls/filter = %d/%d/%d/%#v, want 1/1/1/%#v", store.readTransactionCalls, store.areaExistsCalls, store.listCalls, store.listedFilter, wantFilter)
	}

	projectID := int64(7)
	invalidAreaID := int64(0)
	for _, options := range []ListOptions{
		{Status: ListStatusOpen, AreaID: &invalidAreaID},
		{Status: ListStatusOpen, ProjectID: &projectID, AreaID: &areaID},
	} {
		store := &recordingStore{}
		_, err := NewService(store).List(context.Background(), options)
		if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
			t.Errorf("List(%#v) error = %v, want invalid_argument", options, err)
		}
		if store.listCalls != 0 {
			t.Errorf("store List() calls = %d, want 0", store.listCalls)
		}
	}
}

func TestListValidatesAndResolvesTagFilter(t *testing.T) {
	t.Parallel()

	tagName := "ERRANDS"
	store := &recordingStore{resolveResult: []tag.Tag{{ID: 23, Title: "Errands"}}}
	options := ListOptions{Status: ListStatusOpen, Tag: &tagName}
	if _, err := NewService(store).List(context.Background(), options); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	wantTagID := int64(23)
	wantFilter := ListFilter{Status: ListStatusOpen, TagID: &wantTagID}
	if store.readTransactionCalls != 1 || store.resolveCalls != 1 ||
		!reflect.DeepEqual(store.resolvedNames, []string{tagName}) ||
		store.listCalls != 1 || !reflect.DeepEqual(store.listedFilter, wantFilter) {
		t.Errorf("store read/resolve/names/list/filter = %d/%d/%v/%d/%#v, want 1/1/exact name/1/%#v", store.readTransactionCalls, store.resolveCalls, store.resolvedNames, store.listCalls, store.listedFilter, wantFilter)
	}

	for _, invalid := range []string{" \t\n", string([]byte{0xff})} {
		invalid := invalid
		t.Run("invalid tag", func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			_, err := NewService(store).List(context.Background(), ListOptions{
				Status: ListStatusOpen,
				Tag:    &invalid,
			})
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("List() error = %v, want invalid_argument", err)
			}
			if store.listCalls != 0 {
				t.Errorf("store List() calls = %d, want 0", store.listCalls)
			}
		})
	}
}

func TestListSurfacesReadTransactionErrors(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	tagName := "unknown"
	storeError := errors.New("store failure")
	tests := []struct {
		name  string
		store *recordingStore
	}{
		{
			name:  "transaction boundary",
			store: &recordingStore{readTransactionError: storeError},
		},
		{
			name: "missing project before unknown tag",
			store: &recordingStore{
				projectExistsError: storeError,
				resolveError:       errors.New("must not be returned"),
			},
		},
		{
			name: "tag resolution",
			store: &recordingStore{
				resolveError: storeError,
			},
		},
		{
			name: "list",
			store: &recordingStore{
				resolveResult: []tag.Tag{{ID: 23, Title: "known"}},
				listError:     storeError,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewService(test.store).List(context.Background(), ListOptions{
				Status:    ListStatusAll,
				ProjectID: &projectID,
				Tag:       &tagName,
			})
			if !errors.Is(err, storeError) {
				t.Fatalf("List() error = %v, want store failure", err)
			}
			if test.store.readTransactionCalls != 1 {
				t.Errorf("read transaction calls = %d, want 1", test.store.readTransactionCalls)
			}
		})
	}
}

func TestEditPreservesRequestedFieldsAndNormalizesTimestamp(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	service := NewService(store)
	nowCalls := 0
	service.now = func() time.Time {
		nowCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	title := "  Revised title  "
	note := "line one\nline two\n"
	projectID := int64(9)
	dueOn := "today"
	deferUntil := "tomorrow"
	edited, err := service.Edit(context.Background(), 7, EditRequest{
		Project:    ProjectChange{Set: &projectID},
		Title:      &title,
		Note:       &note,
		DueOn:      DateChange{Set: &dueOn},
		DeferUntil: DateChange{Set: &deferUntil},
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if store.editCalls != 1 || store.editID != 7 {
		t.Errorf("store Edit() calls/ID = %d/%d, want 1/7", store.editCalls, store.editID)
	}
	if store.editFields.Title == nil || *store.editFields.Title != title {
		t.Errorf("edited title = %#v, want exact %q", store.editFields.Title, title)
	}
	if store.editFields.Note == nil || *store.editFields.Note != note {
		t.Errorf("edited note = %#v, want exact %q", store.editFields.Note, note)
	}
	if store.editFields.Project.Set == nil || *store.editFields.Project.Set != projectID ||
		store.editFields.Project.Clear {
		t.Errorf("edited project = %#v, want set to %d", store.editFields.Project, projectID)
	}
	if store.editFields.DueOn.Set == nil || *store.editFields.DueOn.Set != "2026-07-27" {
		t.Errorf("edited due date = %#v, want canonical 2026-07-27", store.editFields.DueOn)
	}
	if store.editFields.DeferUntil.Set == nil || *store.editFields.DeferUntil.Set != "2026-07-28" {
		t.Errorf("edited defer date = %#v, want canonical 2026-07-28", store.editFields.DeferUntil)
	}
	if nowCalls != 1 || store.editTimestamp != "2026-07-27T16:34:56.987Z" || edited.Task.UpdatedAt != store.editTimestamp {
		t.Errorf("clock calls/timestamp = %d/%q, want one call and UTC milliseconds", nowCalls, store.editTimestamp)
	}
}

func TestEditDistinguishesClearedFieldsFromOmittedFields(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	note := ""
	_, err := NewService(store).Edit(context.Background(), 7, EditRequest{
		Project:    ProjectChange{Clear: true},
		Note:       &note,
		DueOn:      DateChange{Clear: true},
		DeferUntil: DateChange{Clear: true},
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if store.editFields.Title != nil {
		t.Errorf("edited title = %#v, want omitted", store.editFields.Title)
	}
	if store.editFields.Note == nil || *store.editFields.Note != "" {
		t.Errorf("edited note = %#v, want explicit empty string", store.editFields.Note)
	}
	if !store.editFields.Project.Clear || store.editFields.Project.Set != nil {
		t.Errorf("edited project = %#v, want explicit clear", store.editFields.Project)
	}
	if !store.editFields.DueOn.Clear || store.editFields.DueOn.Set != nil {
		t.Errorf("edited due date = %#v, want explicit clear", store.editFields.DueOn)
	}
	if !store.editFields.DeferUntil.Clear || store.editFields.DeferUntil.Set != nil {
		t.Errorf("edited defer date = %#v, want explicit clear", store.editFields.DeferUntil)
	}

	omittedStore := &recordingStore{}
	title := "revised"
	_, err = NewService(omittedStore).Edit(context.Background(), 7, EditRequest{Title: &title})
	if err != nil {
		t.Fatalf("Edit(omitted due) error = %v", err)
	}
	if omittedStore.editFields.DueOn != (DateChange{}) {
		t.Errorf("edited due date = %#v, want omitted", omittedStore.editFields.DueOn)
	}
	if omittedStore.editFields.DeferUntil != (DateChange{}) {
		t.Errorf("edited defer date = %#v, want omitted", omittedStore.editFields.DeferUntil)
	}
	if omittedStore.editFields.Project != (ProjectChange{}) {
		t.Errorf("edited project = %#v, want omitted", omittedStore.editFields.Project)
	}
}

func TestEditValidatesAndDelegatesAreaIntent(t *testing.T) {
	t.Parallel()

	areaID := int64(11)
	accepted := []EditRequest{
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

	projectID := int64(7)
	invalidAreaID := int64(0)
	invalid := []EditRequest{
		{Area: AreaChange{Set: &invalidAreaID}},
		{Area: AreaChange{Set: &areaID, Clear: true}},
		{Project: ProjectChange{Set: &projectID}, Area: AreaChange{Set: &areaID}},
	}
	for _, fields := range invalid {
		store := &recordingStore{}
		_, err := NewService(store).Edit(context.Background(), 7, fields)
		if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
			t.Errorf("Edit(%#v) error = %v, want invalid_argument", fields, err)
		}
		if store.editCalls != 0 {
			t.Errorf("store Edit() calls = %d, want 0", store.editCalls)
		}
	}
}

func TestEditRejectsInvalidRequestBeforePersistence(t *testing.T) {
	t.Parallel()

	blankTitle := " \t\n"
	invalidTitle := string([]byte{0xff})
	invalidNote := string([]byte{0xff})
	invalidDate := "next tuesday"
	validDate := "today"
	validTitle := "valid"
	invalidProjectID := int64(0)
	validProjectID := int64(7)
	tests := []struct {
		name   string
		id     int64
		fields EditRequest
	}{
		{name: "nonpositive ID", fields: EditRequest{Title: &validTitle}},
		{name: "no fields", id: 1},
		{name: "blank title", id: 1, fields: EditRequest{Title: &blankTitle}},
		{name: "invalid title UTF-8", id: 1, fields: EditRequest{Title: &invalidTitle}},
		{name: "invalid note UTF-8", id: 1, fields: EditRequest{Note: &invalidNote}},
		{
			name: "nonpositive project ID",
			id:   1,
			fields: EditRequest{Project: ProjectChange{
				Set: &invalidProjectID,
			}},
		},
		{
			name: "set and clear project",
			id:   1,
			fields: EditRequest{Project: ProjectChange{
				Set:   &validProjectID,
				Clear: true,
			}},
		},
		{name: "invalid due date", id: 1, fields: EditRequest{DueOn: DateChange{Set: &invalidDate}}},
		{name: "invalid defer date", id: 1, fields: EditRequest{DeferUntil: DateChange{Set: &invalidDate}}},
		{name: "blank defer stage", id: 1, fields: EditRequest{DeferStage: StageChange{Set: &blankTitle}}},
		{
			name: "set and clear defer stage",
			id:   1,
			fields: EditRequest{DeferStage: StageChange{
				Set:   &validTitle,
				Clear: true,
			}},
		},
		{
			name: "set and clear due date",
			id:   1,
			fields: EditRequest{DueOn: DateChange{
				Set:   &validDate,
				Clear: true,
			}},
		},
		{
			name: "set and clear defer date",
			id:   1,
			fields: EditRequest{DeferUntil: DateChange{
				Set:   &validDate,
				Clear: true,
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			_, err := NewService(store).Edit(context.Background(), test.id, test.fields)
			if err == nil {
				t.Fatal("Edit() error = nil, want invalid_argument")
			}
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("Edit() error = %v, want invalid_argument", err)
			}
			if store.editCalls != 0 || store.transactionCalls != 0 {
				t.Errorf("store Edit()/transaction calls = %d/%d, want 0/0", store.editCalls, store.transactionCalls)
			}
		})
	}
}

func TestEditValidatesDeferStageAgainstTheDestinationProject(t *testing.T) {
	t.Parallel()

	currentProjectID := int64(3)
	destinationProjectID := int64(7)
	projectStageID := int64(11)
	target := StageReference{ID: 13, BoardID: 17, Title: "Waiting", Position: 3}
	notFound := apperr.New(apperr.NotFound, "stage Waiting not found", nil)
	tests := []struct {
		name        string
		transaction *recordingStore
		wantCode    apperr.Code
	}{
		{
			name: "project required",
			transaction: &recordingStore{
				findResults: []Task{{ID: 5}},
			},
			wantCode: apperr.InvalidArgument,
		},
		{
			name: "project must be on board",
			transaction: &recordingStore{
				findResults:       []Task{{ID: 5, ProjectID: &currentProjectID}},
				findProjectResult: domain.Project{ID: currentProjectID},
			},
			wantCode: apperr.InvalidArgument,
		},
		{
			name: "unknown title remains not found",
			transaction: &recordingStore{
				findResults:       []Task{{ID: 5, ProjectID: &currentProjectID}},
				findProjectResult: domain.Project{ID: currentProjectID, StageID: &projectStageID},
				findStageError:    notFound,
			},
			wantCode: apperr.NotFound,
		},
		{
			name: "title only on another board is invalid",
			transaction: &recordingStore{
				findResults:       []Task{{ID: 5, ProjectID: &currentProjectID}},
				findProjectResult: domain.Project{ID: currentProjectID, StageID: &projectStageID},
				findStageError:    notFound,
				stageExistsResult: true,
			},
			wantCode: apperr.InvalidArgument,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outer := &recordingStore{transactionStore: test.transaction}
			_, err := NewService(outer).Edit(context.Background(), 5, EditRequest{
				DeferStage: StageChange{Set: &target.Title},
			})
			if code, ok := apperr.CodeOf(err); !ok || code != test.wantCode {
				t.Fatalf("Edit() error = %v, want %s", err, test.wantCode)
			}
			if outer.transactionCalls != 1 || outer.editCalls != 0 || test.transaction.editCalls != 0 {
				t.Errorf("transaction/outer edit/transaction edit calls = %d/%d/%d, want 1/0/0", outer.transactionCalls, outer.editCalls, test.transaction.editCalls)
			}
		})
	}

	oldDeferStageID := int64(9)
	editedTask := Task{ID: 5, ProjectID: &destinationProjectID, DeferStageID: &target.ID}
	transaction := &recordingStore{
		findResults:         []Task{{ID: 5, ProjectID: &currentProjectID, DeferStageID: &oldDeferStageID}},
		findProjectResult:   domain.Project{ID: destinationProjectID, StageID: &projectStageID},
		findStageByIDResult: StageReference{ID: projectStageID, BoardID: target.BoardID},
		findStageResult:     target,
		editResult:          &editedTask,
	}
	outer := &recordingStore{transactionStore: transaction}
	result, err := NewService(outer).Edit(context.Background(), 5, EditRequest{
		Project:    ProjectChange{Set: &destinationProjectID},
		DeferStage: StageChange{Set: &target.Title},
	})
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if outer.transactionCalls != 1 || transaction.findCalls != 1 || transaction.findProjectID != destinationProjectID ||
		transaction.findStageByIDCalls != 1 || transaction.findStageProjectID != target.BoardID ||
		transaction.editCalls != 1 {
		t.Errorf("destination-stage edit calls = %#v, want destination project %d resolved in one transaction", transaction, destinationProjectID)
	}
	if transaction.editFields.DeferStageID.Set == nil || *transaction.editFields.DeferStageID.Set != target.ID ||
		transaction.editFields.DeferStageID.Clear {
		t.Errorf("persisted defer stage = %#v, want explicit stage %d", transaction.editFields.DeferStageID, target.ID)
	}
	if !reflect.DeepEqual(result.Task, editedTask) || len(result.ClearedDefers) != 0 {
		t.Errorf("Edit() = %#v, want edited task with no implicit clear report", result)
	}
}

func TestEditClearsStageOnlyForAnActualReparentWithoutExplicitStage(t *testing.T) {
	t.Parallel()

	currentProjectID := int64(3)
	destinationProjectID := int64(7)
	deferStageID := int64(11)
	tests := []struct {
		name        string
		destination int64
		wantClear   bool
	}{
		{name: "actual reparent clears", destination: destinationProjectID, wantClear: true},
		{name: "redundant project restatement preserves", destination: currentProjectID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			editedTask := Task{ID: 5, ProjectID: &test.destination}
			if !test.wantClear {
				editedTask.DeferStageID = &deferStageID
			}
			transaction := &recordingStore{
				findResults: []Task{{ID: 5, ProjectID: &currentProjectID, DeferStageID: &deferStageID}},
				editResult:  &editedTask,
			}
			outer := &recordingStore{transactionStore: transaction}
			result, err := NewService(outer).Edit(context.Background(), 5, EditRequest{
				Project: ProjectChange{Set: &test.destination},
			})
			if err != nil {
				t.Fatalf("Edit() error = %v", err)
			}
			if outer.transactionCalls != 1 || transaction.editCalls != 1 ||
				transaction.editFields.DeferStageID.Clear != test.wantClear {
				t.Errorf("transaction/edit/stage clear = %d/%d/%t, want 1/1/%t", outer.transactionCalls, transaction.editCalls, transaction.editFields.DeferStageID.Clear, test.wantClear)
			}
			if test.wantClear {
				if !reflect.DeepEqual(result.ClearedDefers, []Task{editedTask}) {
					t.Errorf("cleared defers = %#v, want edited task", result.ClearedDefers)
				}
			} else if len(result.ClearedDefers) != 0 {
				t.Errorf("cleared defers = %#v, want empty", result.ClearedDefers)
			}
		})
	}
}

func TestEditPropagatesPromotesSetAndClear(t *testing.T) {
	t.Parallel()

	for _, value := range []bool{true, false} {
		value := value
		t.Run(map[bool]string{true: "set", false: "clear"}[value], func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			result, err := NewService(store).Edit(context.Background(), 5, EditRequest{Promotes: &value})
			if err != nil {
				t.Fatalf("Edit() error = %v", err)
			}
			if store.editCalls != 1 || store.transactionCalls != 0 || store.editFields.Promotes == nil ||
				*store.editFields.Promotes != value {
				t.Errorf("edit calls/transaction/promotes = %d/%d/%#v, want 1/0/%t", store.editCalls, store.transactionCalls, store.editFields.Promotes, value)
			}
			if result.ClearedDefers == nil {
				t.Error("Edit() cleared defers = nil, want JSON-stable empty list")
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
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
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

	placement := domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 11}
	want := Task{ID: 7, Title: "moved", Position: 2}
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

func TestTagAndUntagOwnNormalizedTransactionsAndReturnStoredTitles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		apply        func(*Service) (Tagging, error)
		mutationTags func(*recordingStore) []tag.Tag
		mutationID   func(*recordingStore) int64
	}{
		{
			name: "tag",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"Errands", "ERRANDS", "é", "É"})
			},
			mutationTags: func(store *recordingStore) []tag.Tag { return store.attachedTags },
			mutationID:   func(store *recordingStore) int64 { return store.attachedID },
		},
		{
			name: "untag",
			apply: func(service *Service) (Tagging, error) {
				return service.Untag(context.Background(), 7, []string{"Errands", "ERRANDS", "é", "É"})
			},
			mutationTags: func(store *recordingStore) []tag.Tag { return store.detachedTags },
			mutationID:   func(store *recordingStore) int64 { return store.detachedID },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resolved := []tag.Tag{
				{ID: 1, Title: "Stored Errands"},
				{ID: 2, Title: "é"},
				{ID: 3, Title: "É"},
			}
			refreshed := Task{ID: 7, Title: "task", Tags: []string{"Stored Errands", "é", "É"}}
			transactionStore := &recordingStore{
				findResults:   []Task{{ID: 7, Title: "task"}, refreshed},
				resolveResult: resolved,
			}
			store := &recordingStore{transactionStore: transactionStore}

			got, err := test.apply(NewService(store))
			if err != nil {
				t.Fatalf("%s() error = %v", test.name, err)
			}
			want := Tagging{Task: refreshed, TagTitles: []string{"Stored Errands", "é", "É"}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s() = %#v, want %#v", test.name, got, want)
			}
			if store.transactionCalls != 1 || store.findCalls+store.resolveCalls+store.attachCalls+store.detachCalls != 0 {
				t.Errorf("outer store calls = %#v, want transaction boundary only", store)
			}
			wantNames := []string{"Errands", "é", "É"}
			if !reflect.DeepEqual(transactionStore.resolvedNames, wantNames) {
				t.Errorf("resolved names = %v, want %v", transactionStore.resolvedNames, wantNames)
			}
			if transactionStore.findCalls != 2 || transactionStore.resolveCalls != 1 ||
				transactionStore.attachCalls+transactionStore.detachCalls != 1 {
				t.Errorf("transaction-scoped calls = %#v, want two finds, one resolve, and one mutation", transactionStore)
			}
			if test.mutationID(transactionStore) != 7 || !reflect.DeepEqual(test.mutationTags(transactionStore), resolved) {
				t.Errorf(
					"mutation ID/tags = %d/%#v, want 7/%#v",
					test.mutationID(transactionStore),
					test.mutationTags(transactionStore),
					resolved,
				)
			}
		})
	}
}

func TestTagAndUntagValidateBeforeTransaction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{name: "tag ID", apply: func(service *Service) error {
			_, err := service.Tag(context.Background(), 0, []string{"known"})
			return err
		}},
		{name: "untag ID", apply: func(service *Service) error {
			_, err := service.Untag(context.Background(), 0, []string{"known"})
			return err
		}},
		{name: "tag names", apply: func(service *Service) error { _, err := service.Tag(context.Background(), 1, nil); return err }},
		{name: "untag names", apply: func(service *Service) error { _, err := service.Untag(context.Background(), 1, []string{}); return err }},
		{name: "tag blank name", apply: func(service *Service) error {
			_, err := service.Tag(context.Background(), 1, []string{" "})
			return err
		}},
		{name: "untag invalid UTF-8", apply: func(service *Service) error {
			_, err := service.Untag(context.Background(), 1, []string{string([]byte{0xff})})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			err := test.apply(NewService(store))
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("mutation error = %v, want invalid_argument", err)
			}
			if !strings.Contains(err.Error(), "must") && !strings.Contains(err.Error(), "at least one") {
				t.Errorf("mutation error = %q, want semantic validation message", err)
			}
			if store.transactionCalls != 0 {
				t.Errorf("transaction calls = %d, want 0", store.transactionCalls)
			}
		})
	}
}

func TestTagAndUntagReturnTransactionErrors(t *testing.T) {
	t.Parallel()

	storeError := errors.New("store failure")
	tests := []struct {
		name        string
		apply       func(*Service) (Tagging, error)
		transaction *recordingStore
		outerError  error
	}{
		{
			name: "tag transaction boundary",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"known"})
			},
			transaction: &recordingStore{},
			outerError:  storeError,
		},
		{
			name: "tag entity lookup",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"known"})
			},
			transaction: &recordingStore{findError: storeError},
		},
		{
			name: "tag resolution",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"unknown"})
			},
			transaction: &recordingStore{resolveError: storeError},
		},
		{
			name: "tag attach",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"known"})
			},
			transaction: &recordingStore{
				resolveResult: []tag.Tag{{ID: 1, Title: "known"}},
				attachError:   storeError,
			},
		},
		{
			name: "tag refresh",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, []string{"known"})
			},
			transaction: &recordingStore{
				resolveResult: []tag.Tag{{ID: 1, Title: "known"}},
				findErrors:    []error{nil, storeError},
			},
		},
		{
			name: "untag detach",
			apply: func(service *Service) (Tagging, error) {
				return service.Untag(context.Background(), 7, []string{"known"})
			},
			transaction: &recordingStore{
				resolveResult: []tag.Tag{{ID: 1, Title: "known"}},
				detachError:   storeError,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{
				transactionStore: test.transaction,
				transactionError: test.outerError,
			}
			_, err := test.apply(NewService(store))
			if !errors.Is(err, storeError) {
				t.Fatalf("mutation error = %v, want store failure", err)
			}
			if store.transactionCalls != 1 || store.findCalls+store.resolveCalls+store.attachCalls+store.detachCalls != 0 {
				t.Errorf("outer store calls = %#v, want transaction boundary only", store)
			}
		})
	}
}

func TestDonePromotesMarkedProjectExactlyOneStage(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	currentStageID := int64(11)
	nextStageID := int64(13)
	completed := Task{ID: 5, ProjectID: &projectID, Promotes: true}
	currentProject := domain.Project{ID: projectID, StageID: &currentStageID}
	currentStage := StageReference{ID: currentStageID, BoardID: 17, Title: "Doing", Position: 2}
	nextStage := StageReference{ID: nextStageID, BoardID: 17, Title: "Done", Position: 3}
	promotedProject := domain.Project{ID: projectID, StageID: &nextStageID}
	transaction := &recordingStore{
		doneResult:          &completed,
		findProjectResult:   currentProject,
		findStageByIDResult: currentStage,
		findNextStageResult: &nextStage,
		moveProjectResult:   promotedProject,
	}
	outer := &recordingStore{transactionStore: transaction}
	result, err := NewService(outer).Done(context.Background(), 5)
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if outer.transactionCalls != 1 || outer.doneCalls != 0 || transaction.doneCalls != 1 ||
		transaction.findProjectCalls != 1 || transaction.findProjectID != projectID ||
		transaction.findStageByIDCalls != 1 || transaction.findStageByIDID != currentStageID ||
		transaction.findNextStageCalls != 1 || transaction.findNextStageProjectID != currentStage.BoardID ||
		transaction.findNextStageCurrentID != currentStage.Position || transaction.moveProjectStageCalls != 1 ||
		transaction.moveProjectID != projectID || transaction.moveProjectStageID != nextStageID {
		t.Errorf("promotion orchestration = %#v, want exactly one next-stage move in one transaction", transaction)
	}
	if !reflect.DeepEqual(result.Task, completed) || result.PromotedProject == nil ||
		!reflect.DeepEqual(*result.PromotedProject, promotedProject) || result.Promotion == nil ||
		result.Promotion.StageTitle != nextStage.Title || result.Promotion.LastStage {
		t.Errorf("Done() = %#v, want completed task and promoted project metadata", result)
	}
}

func TestDoneReportsLastStageWithoutJSONPromotion(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	stageID := int64(11)
	completed := Task{ID: 5, ProjectID: &projectID, Promotes: true}
	project := domain.Project{ID: projectID, StageID: &stageID}
	stage := StageReference{ID: stageID, BoardID: 17, Title: "Done", Position: 3}
	transaction := &recordingStore{
		doneResult:          &completed,
		findProjectResult:   project,
		findStageByIDResult: stage,
	}
	outer := &recordingStore{transactionStore: transaction}
	result, err := NewService(outer).Done(context.Background(), 5)
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if transaction.findNextStageCalls != 1 || transaction.moveProjectStageCalls != 0 {
		t.Errorf("next/move calls = %d/%d, want 1/0", transaction.findNextStageCalls, transaction.moveProjectStageCalls)
	}
	if result.PromotedProject != nil || result.Promotion == nil || !result.Promotion.LastStage ||
		result.Promotion.StageTitle != stage.Title || !reflect.DeepEqual(result.Promotion.Project, project) {
		t.Errorf("Done() = %#v, want nil JSON promotion and last-stage human metadata", result)
	}
}

func TestDonePromotionIsInertForOffBoardProject(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	completed := Task{ID: 5, ProjectID: &projectID, Promotes: true}
	transaction := &recordingStore{
		doneResult:        &completed,
		findProjectResult: domain.Project{ID: projectID},
	}
	outer := &recordingStore{transactionStore: transaction}
	result, err := NewService(outer).Done(context.Background(), 5)
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if transaction.findProjectCalls != 1 || transaction.findStageByIDCalls != 0 ||
		transaction.findNextStageCalls != 0 || transaction.moveProjectStageCalls != 0 {
		t.Errorf("off-board promotion calls = %#v, want project lookup only", transaction)
	}
	if result.PromotedProject != nil || result.Promotion != nil {
		t.Errorf("Done() = %#v, want no promotion", result)
	}
}

func TestDoneReturnsStageWriteFailureThroughOneTransaction(t *testing.T) {
	t.Parallel()

	storeError := errors.New("stage write failed")
	projectID := int64(7)
	currentStageID := int64(11)
	nextStageID := int64(13)
	transaction := &recordingStore{
		doneResult:          &Task{ID: 5, ProjectID: &projectID, Promotes: true},
		findProjectResult:   domain.Project{ID: projectID, StageID: &currentStageID},
		findStageByIDResult: StageReference{ID: currentStageID, BoardID: 17, Position: 2},
		findNextStageResult: &StageReference{ID: nextStageID, BoardID: 17, Position: 3},
		moveProjectError:    storeError,
	}
	outer := &recordingStore{transactionStore: transaction}
	result, err := NewService(outer).Done(context.Background(), 5)
	if !errors.Is(err, storeError) {
		t.Fatalf("Done() error = %v, want stage write failure", err)
	}
	if !reflect.DeepEqual(result, Completion{}) {
		t.Errorf("Done() = %#v, want zero result on transaction failure", result)
	}
	if outer.transactionCalls != 1 || outer.doneCalls != 0 || outer.moveProjectStageCalls != 0 ||
		transaction.doneCalls != 1 || transaction.moveProjectStageCalls != 1 {
		t.Errorf("outer/transaction orchestration = %#v/%#v, want both writes attempted through one transaction", outer, transaction)
	}
}

func TestReopenDoesNotRunPromotionOrDemotionOrchestration(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	result, err := NewService(store).Reopen(context.Background(), 5)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if result.ID != 5 || store.reopenCalls != 1 || store.transactionCalls != 0 ||
		store.findProjectCalls != 0 || store.findStageByIDCalls != 0 ||
		store.findNextStageCalls != 0 || store.moveProjectStageCalls != 0 {
		t.Errorf("Reopen() result/store = %#v/%#v, want unchanged direct reopen with no demotion", result, store)
	}
}

func TestLifecycleValidatesIDBeforePersistence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
		calls func(*recordingStore) int
	}{
		{name: "done", apply: func(service *Service) error { _, err := service.Done(context.Background(), 0); return err }, calls: func(store *recordingStore) int { return store.doneCalls }},
		{name: "cancel", apply: func(service *Service) error { _, err := service.Cancel(context.Background(), 0); return err }, calls: func(store *recordingStore) int { return store.cancelCalls }},
		{name: "reopen", apply: func(service *Service) error { _, err := service.Reopen(context.Background(), 0); return err }, calls: func(store *recordingStore) int { return store.reopenCalls }},
		{name: "delete", apply: func(service *Service) error { _, err := service.Delete(context.Background(), 0); return err }, calls: func(store *recordingStore) int { return store.deleteCalls }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := &recordingStore{}
			err := test.apply(NewService(store))
			if err == nil {
				t.Fatal("lifecycle error = nil, want invalid_argument")
			}
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("lifecycle error = %v, want invalid_argument", err)
			}
			if calls := test.calls(store); calls != 0 {
				t.Errorf("store calls = %d, want 0", calls)
			}
		})
	}
}

func TestLifecycleDelegatesOneNormalizedTimestampPerAttempt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		apply func(*Service) error
		calls func(*recordingStore) int
	}{
		{name: "done", apply: func(service *Service) error { _, err := service.Done(context.Background(), 7); return err }, calls: func(store *recordingStore) int { return store.doneCalls }},
		{name: "cancel", apply: func(service *Service) error { _, err := service.Cancel(context.Background(), 7); return err }, calls: func(store *recordingStore) int { return store.cancelCalls }},
		{name: "reopen", apply: func(service *Service) error { _, err := service.Reopen(context.Background(), 7); return err }, calls: func(store *recordingStore) int { return store.reopenCalls }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store := &recordingStore{}
			service := NewService(store)
			nowCalls := 0
			service.now = func() time.Time {
				nowCalls++
				return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
			}

			if err := test.apply(service); err != nil {
				t.Fatalf("lifecycle error = %v", err)
			}
			if test.calls(store) != 1 || store.lifecycleID != 7 {
				t.Errorf("store calls/ID = %d/%d, want 1/7", test.calls(store), store.lifecycleID)
			}
			if nowCalls != 1 || store.lifecycleTimestamp != "2026-07-27T16:34:56.987Z" {
				t.Errorf("now calls/timestamp = %d/%q, want 1/UTC milliseconds", nowCalls, store.lifecycleTimestamp)
			}
		})
	}
}
