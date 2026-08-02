package area

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type recordingStore struct {
	addCalls             int
	addFields            AddFields
	addTimestamp         string
	addResult            Area
	addError             error
	findCalls            int
	findID               int64
	findResult           Area
	findError            error
	listCalls            int
	listOptions          ListOptions
	listResult           []Area
	listError            error
	editCalls            int
	editID               int64
	editFields           EditFields
	editTimestamp        string
	editResult           Area
	editError            error
	archiveCalls         int
	archiveID            int64
	archiveTimestamp     string
	archiveResult        Area
	archiveError         error
	unarchiveCalls       int
	unarchiveID          int64
	unarchiveTimestamp   string
	unarchiveResult      Area
	unarchiveError       error
	deleteCalls          int
	deleteID             int64
	deleteResult         Area
	deleteError          error
	deleteProjectsCalls  int
	deleteProjectsAreaID int64
	deleteProjectsResult []project.Project
	deleteTasksCalls     int
	deleteTasksAreaIDs   []int64
	deleteTasksScopes    []TaskDeletionScope
	projectTasksResult   []task.Task
	looseTasksResult     []task.Task
	transactionCalls     int
	transactionStore     Store
	transactionError     error
	lifecycleSequence    []string
}

func (r *recordingStore) Add(
	_ context.Context,
	fields AddFields,
	timestamp string,
) (Area, error) {
	r.addCalls++
	r.addFields = fields
	r.addTimestamp = timestamp
	return r.addResult, r.addError
}

func (r *recordingStore) Find(_ context.Context, id int64) (Area, error) {
	r.findCalls++
	r.findID = id
	return r.findResult, r.findError
}

func (r *recordingStore) List(_ context.Context, options ListOptions) ([]Area, error) {
	r.listCalls++
	r.listOptions = options
	return r.listResult, r.listError
}

func (r *recordingStore) Edit(
	_ context.Context,
	id int64,
	fields EditFields,
	timestamp string,
) (Area, error) {
	r.editCalls++
	r.editID = id
	r.editFields = fields
	r.editTimestamp = timestamp
	return r.editResult, r.editError
}

func (r *recordingStore) Archive(_ context.Context, id int64, timestamp string) (Area, error) {
	r.archiveCalls++
	r.archiveID = id
	r.archiveTimestamp = timestamp
	return r.archiveResult, r.archiveError
}

func (r *recordingStore) Unarchive(_ context.Context, id int64, timestamp string) (Area, error) {
	r.unarchiveCalls++
	r.unarchiveID = id
	r.unarchiveTimestamp = timestamp
	return r.unarchiveResult, r.unarchiveError
}

func (r *recordingStore) Delete(_ context.Context, id int64) (Area, error) {
	r.deleteCalls++
	r.deleteID = id
	r.lifecycleSequence = append(r.lifecycleSequence, "delete area")
	return r.deleteResult, r.deleteError
}

func (r *recordingStore) DeleteProjects(
	_ context.Context,
	areaID int64,
) ([]project.Project, error) {
	r.deleteProjectsCalls++
	r.deleteProjectsAreaID = areaID
	r.lifecycleSequence = append(r.lifecycleSequence, "delete projects")
	return r.deleteProjectsResult, nil
}

func (r *recordingStore) DeleteTasks(
	_ context.Context,
	areaID int64,
	scope TaskDeletionScope,
) ([]task.Task, error) {
	r.deleteTasksCalls++
	r.deleteTasksAreaIDs = append(r.deleteTasksAreaIDs, areaID)
	r.deleteTasksScopes = append(r.deleteTasksScopes, scope)
	r.lifecycleSequence = append(r.lifecycleSequence, "delete "+string(scope)+" tasks")
	if scope == TaskDeletionScopeProject {
		return r.projectTasksResult, nil
	}
	return r.looseTasksResult, nil
}

func (r *recordingStore) WithinTransaction(
	ctx context.Context,
	operation func(Store) error,
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

func TestAddPreservesAcceptedTextAndUsesOneUTCMillisecondTimestamp(t *testing.T) {
	t.Parallel()

	fields := AddFields{Title: "  Home  ", Note: "line one\nline two\n"}
	want := Area{ID: 1, Title: fields.Title, Note: fields.Note}
	store := &recordingStore{addResult: want}
	service := NewService(store)
	clockCalls := 0
	service.now = func() time.Time {
		clockCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	got, err := service.Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) || store.addCalls != 1 || store.addFields != fields {
		t.Errorf("Add() result/calls/fields = %#v/%d/%#v, want %#v/1/%#v", got, store.addCalls, store.addFields, want, fields)
	}
	if clockCalls != 1 || store.addTimestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want 1/UTC milliseconds", clockCalls, store.addTimestamp)
	}
}

func TestServiceRejectsInvalidInputsBeforePersistence(t *testing.T) {
	t.Parallel()

	validTitle := "valid"
	blankTitle := " \t\n"
	invalidText := string([]byte{0xff})
	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{name: "blank add title", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), AddFields{Title: blankTitle})
			return err
		}},
		{name: "invalid add title UTF-8", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), AddFields{Title: invalidText})
			return err
		}},
		{name: "invalid add note UTF-8", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), AddFields{Title: validTitle, Note: invalidText})
			return err
		}},
		{name: "nonpositive show ID", apply: func(service *Service) error {
			_, err := service.Show(context.Background(), 0)
			return err
		}},
		{name: "nonpositive edit ID", apply: func(service *Service) error {
			_, err := service.Edit(context.Background(), 0, EditFields{Title: &validTitle})
			return err
		}},
		{name: "nonpositive archive ID", apply: func(service *Service) error {
			_, err := service.Archive(context.Background(), 0)
			return err
		}},
		{name: "nonpositive unarchive ID", apply: func(service *Service) error {
			_, err := service.Unarchive(context.Background(), 0)
			return err
		}},
		{name: "nonpositive delete ID", apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), 0, true)
			return err
		}},
		{name: "edit without fields", apply: func(service *Service) error {
			_, err := service.Edit(context.Background(), 1, EditFields{})
			return err
		}},
		{name: "blank edit title", apply: func(service *Service) error {
			_, err := service.Edit(context.Background(), 1, EditFields{Title: &blankTitle})
			return err
		}},
		{name: "invalid edit title UTF-8", apply: func(service *Service) error {
			_, err := service.Edit(context.Background(), 1, EditFields{Title: &invalidText})
			return err
		}},
		{name: "invalid edit note UTF-8", apply: func(service *Service) error {
			_, err := service.Edit(context.Background(), 1, EditFields{Note: &invalidText})
			return err
		}},
		{name: "invalid list slice", apply: func(service *Service) error {
			_, err := service.List(context.Background(), ListOptions{Slice: ListSlice("ACTIVE")})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			if err := test.apply(NewService(store)); errorCode(err) != apperr.InvalidArgument {
				t.Errorf("operation error = %v, want invalid_argument", err)
			}
			if store.addCalls+store.findCalls+store.listCalls+store.editCalls+
				store.archiveCalls+store.unarchiveCalls+store.deleteCalls+store.transactionCalls != 0 {
				t.Errorf("store received a call after invalid input: %#v", store)
			}
		})
	}
}

func TestParseIDIsStrict(t *testing.T) {
	t.Parallel()

	if id, err := ParseID("001"); err != nil || id != 1 {
		t.Fatalf("ParseID(001) = %d, %v, want 1, nil", id, err)
	}
	for _, value := range []string{"", "0", "-1", "+1", "1.0", "１２", "9223372036854775808"} {
		if _, err := ParseID(value); errorCode(err) != apperr.InvalidArgument {
			t.Errorf("ParseID(%q) error = %v, want invalid_argument", value, err)
		}
	}
}

func TestListDefaultsToActiveAndNormalizesNil(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	listed, err := NewService(store).List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("List() = %#v, want non-nil empty list", listed)
	}
	if store.listCalls != 1 || store.listOptions.Slice != ListSliceActive {
		t.Errorf("store list calls/options = %d/%#v, want 1/active", store.listCalls, store.listOptions)
	}
}

func TestShowAndEditDelegateExactIntentAndStoreErrors(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	title := "  Revised  "
	note := "details\n"
	store := &recordingStore{findError: storeError}
	service := NewService(store)
	if _, err := service.Show(context.Background(), 7); !errors.Is(err, storeError) {
		t.Errorf("Show() error = %v, want preserved %v", err, storeError)
	}
	if store.findCalls != 1 || store.findID != 7 {
		t.Errorf("Find() calls/ID = %d/%d, want 1/7", store.findCalls, store.findID)
	}

	store.findError = nil
	store.editError = storeError
	clockCalls := 0
	service.now = func() time.Time {
		clockCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.UTC)
	}
	fields := EditFields{Title: &title, Note: &note}
	if _, err := service.Edit(context.Background(), 7, fields); !errors.Is(err, storeError) {
		t.Errorf("Edit() error = %v, want preserved %v", err, storeError)
	}
	if store.editCalls != 1 || store.editID != 7 || !reflect.DeepEqual(store.editFields, fields) {
		t.Errorf("Edit() calls/ID/fields = %d/%d/%#v, want 1/7/%#v", store.editCalls, store.editID, store.editFields, fields)
	}
	if clockCalls != 1 || store.editTimestamp != "2026-07-27T12:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want 1/UTC milliseconds", clockCalls, store.editTimestamp)
	}

	store.listError = storeError
	if _, err := service.List(context.Background(), ListOptions{Slice: ListSliceAll}); !errors.Is(err, storeError) {
		t.Errorf("List() error = %v, want preserved %v", err, storeError)
	}
	store.addError = storeError
	if _, err := service.Add(context.Background(), AddFields{Title: "valid"}); !errors.Is(err, storeError) {
		t.Errorf("Add() error = %v, want preserved %v", err, storeError)
	}
}

func TestArchiveAndUnarchiveDelegateTimestamps(t *testing.T) {
	t.Parallel()

	want := Area{ID: 7, Title: "Home"}
	store := &recordingStore{archiveResult: want, unarchiveResult: want}
	service := NewService(store)
	clockCalls := 0
	service.now = func() time.Time {
		clockCalls++
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.FixedZone("offset", -4*60*60))
	}

	archived, archiveErr := service.Archive(context.Background(), 7)
	unarchived, unarchiveErr := service.Unarchive(context.Background(), 7)
	if archiveErr != nil || unarchiveErr != nil {
		t.Fatalf("Archive()/Unarchive() errors = %v/%v", archiveErr, unarchiveErr)
	}
	if archived != want || unarchived != want || store.archiveCalls != 1 ||
		store.unarchiveCalls != 1 || store.archiveID != 7 || store.unarchiveID != 7 {
		t.Errorf("lifecycle results/store = %#v/%#v/%#v, want exact area and ID delegation", archived, unarchived, store)
	}
	wantTimestamp := "2026-07-27T16:34:56.987Z"
	if clockCalls != 2 || store.archiveTimestamp != wantTimestamp || store.unarchiveTimestamp != wantTimestamp {
		t.Errorf(
			"clock/timestamps = %d/%q/%q, want two calls and %q",
			clockCalls,
			store.archiveTimestamp,
			store.unarchiveTimestamp,
			wantTimestamp,
		)
	}
}

func TestNonrecursiveDeleteReturnsNormalizedEnvelopeWithoutTransaction(t *testing.T) {
	t.Parallel()

	deletedArea := Area{ID: 7, Title: "Empty"}
	store := &recordingStore{deleteResult: deletedArea}
	deletion, err := NewService(store).Delete(context.Background(), 7, false)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.deleteCalls != 1 || store.deleteID != 7 || store.transactionCalls != 0 ||
		store.deleteProjectsCalls != 0 || store.deleteTasksCalls != 0 {
		t.Errorf("delete delegation = %#v, want one direct area delete", store)
	}
	if deletion.Area != deletedArea || deletion.DeletedProjects == nil ||
		len(deletion.DeletedProjects) != 0 || deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
		t.Errorf("Delete() = %#v, want area and non-nil empty arrays", deletion)
	}
}

func TestRecursiveDeleteDeletesAreaLastAndReturnsContainerGroupedEnvelope(t *testing.T) {
	t.Parallel()

	deletedArea := Area{ID: 7, Title: "Home"}
	firstProjectID := int64(10)
	secondProjectID := int64(11)
	transactionStore := &recordingStore{
		deleteResult: deletedArea,
		deleteProjectsResult: []project.Project{
			{ID: 10, Position: 1},
			{ID: 11, Position: 1},
			{ID: 12, Position: 2},
		},
		projectTasksResult: []task.Task{
			{ID: 20, ProjectID: &secondProjectID, Position: 0},
			{ID: 22, ProjectID: &firstProjectID, Position: 2},
		},
		looseTasksResult: []task.Task{
			{ID: 21, Position: 0},
			{ID: 23, Position: 1},
		},
	}
	store := &recordingStore{transactionStore: transactionStore}

	deletion, err := NewService(store).Delete(context.Background(), 7, true)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.transactionCalls != 1 ||
		store.deleteCalls+store.deleteProjectsCalls+store.deleteTasksCalls != 0 {
		t.Errorf("outer transaction/direct mutation state = %#v, want one boundary only", store)
	}
	sequence := transactionStore.lifecycleSequence
	if len(sequence) != 4 || sequence[len(sequence)-1] != "delete area" {
		t.Errorf("mutation sequence = %v, want the area deleted after its contents", sequence)
	}
	if transactionStore.deleteProjectsAreaID != 7 ||
		!reflect.DeepEqual(transactionStore.deleteTasksAreaIDs, []int64{7, 7}) ||
		!reflect.DeepEqual(
			transactionStore.deleteTasksScopes,
			[]TaskDeletionScope{TaskDeletionScopeProject, TaskDeletionScopeLoose},
		) || transactionStore.deleteID != 7 {
		t.Errorf("recursive delete intent = %#v, want area 7 with project then loose scopes", transactionStore)
	}
	if deletion.Area != deletedArea {
		t.Errorf("deleted area = %#v, want %#v", deletion.Area, deletedArea)
	}
	wantProjects := []int64{10, 11, 12}
	gotProjects := make([]int64, len(deletion.DeletedProjects))
	for index, deletedProject := range deletion.DeletedProjects {
		gotProjects[index] = deletedProject.ID
	}
	if !reflect.DeepEqual(gotProjects, wantProjects) {
		t.Errorf("deleted project IDs = %v, want store order %v preserved", gotProjects, wantProjects)
	}
	wantTasks := []int64{21, 23, 22, 20}
	gotTasks := make([]int64, len(deletion.DeletedTasks))
	for index, deletedTask := range deletion.DeletedTasks {
		gotTasks[index] = deletedTask.ID
	}
	if !reflect.DeepEqual(gotTasks, wantTasks) {
		t.Errorf("deleted task IDs = %v, want loose tasks then per-project groups %v", gotTasks, wantTasks)
	}
}

func TestRecursiveDeleteNormalizesNilCollections(t *testing.T) {
	t.Parallel()

	transactionStore := &recordingStore{deleteResult: Area{ID: 7}}
	store := &recordingStore{transactionStore: transactionStore}
	deletion, err := NewService(store).Delete(context.Background(), 7, true)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deletion.DeletedProjects == nil || len(deletion.DeletedProjects) != 0 ||
		deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
		t.Errorf("Delete() collections = %#v/%#v, want non-nil empty arrays", deletion.DeletedProjects, deletion.DeletedTasks)
	}
}

func TestAreaLifecycleStoreAndTransactionErrorsPassThrough(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	tests := []struct {
		name  string
		store *recordingStore
		apply func(*Service) error
	}{
		{name: "archive", store: &recordingStore{archiveError: storeError}, apply: func(service *Service) error {
			_, err := service.Archive(context.Background(), 7)
			return err
		}},
		{name: "unarchive", store: &recordingStore{unarchiveError: storeError}, apply: func(service *Service) error {
			_, err := service.Unarchive(context.Background(), 7)
			return err
		}},
		{name: "direct delete", store: &recordingStore{deleteError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), 7, false)
			return err
		}},
		{name: "transaction boundary", store: &recordingStore{transactionError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), 7, true)
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if err := test.apply(NewService(test.store)); !errors.Is(err, storeError) {
				t.Errorf("operation error = %v, want preserved %v", err, storeError)
			}
		})
	}
}

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
