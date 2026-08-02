package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
)

type recordingStore struct {
	addCalls             int
	addFields            AddFields
	addTimestamp         string
	addError             error
	findCalls            int
	findID               int64
	findError            error
	listCalls            int
	listOptions          ListOptions
	listResult           []Project
	listError            error
	editCalls            int
	editID               int64
	editFields           EditFields
	editTimestamp        string
	editError            error
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
	transactionCalls     int
	transactionStore     Store
	lifecycleSequence    []string
}

func (r *recordingStore) Add(
	_ context.Context,
	fields AddFields,
	timestamp string,
) (Project, error) {
	r.addCalls++
	r.addFields = fields
	r.addTimestamp = timestamp

	return Project{
		ID:        1,
		AreaID:    fields.AreaID,
		Title:     fields.Title,
		Note:      fields.Note,
		Status:    string(ListStatusOpen),
		CreatedAt: timestamp,
		UpdatedAt: timestamp,
	}, r.addError
}

func (r *recordingStore) Find(_ context.Context, id int64) (Project, error) {
	r.findCalls++
	r.findID = id
	return Project{ID: id}, r.findError
}

func (r *recordingStore) List(_ context.Context, options ListOptions) ([]Project, error) {
	r.listCalls++
	r.listOptions = options
	return r.listResult, r.listError
}

func (r *recordingStore) Edit(
	_ context.Context,
	id int64,
	fields EditFields,
	timestamp string,
) (Project, error) {
	r.editCalls++
	r.editID = id
	r.editFields = fields
	r.editTimestamp = timestamp

	return Project{ID: id, UpdatedAt: timestamp}, r.editError
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
	r.lifecycleSequence = append(r.lifecycleSequence, "resolve")

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
	r.lifecycleSequence = append(r.lifecycleSequence, "cancel open tasks")
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
	r.lifecycleSequence = append(r.lifecycleSequence, "delete project")

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
	r.lifecycleSequence = append(r.lifecycleSequence, "delete tasks")
	return r.deleteTasksResult, r.deleteTasksError
}

func (r *recordingStore) WithinTransaction(
	ctx context.Context,
	operation func(Store) error,
) error {
	r.transactionCalls++
	store := r.transactionStore
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
	if store.addCalls != 1 || store.addFields != fields {
		t.Errorf("store Add() calls/fields = %d/%#v, want 1/%#v", store.addCalls, store.addFields, fields)
	}
	if created.Title != fields.Title || created.Note != fields.Note {
		t.Errorf("Add() = %#v, want exact accepted text", created)
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
	if store.addCalls != 1 || store.addFields != fields ||
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			_, err := NewService(store).Add(context.Background(), test.fields)
			if errorCode(err) != apperr.InvalidArgument {
				t.Errorf("Add() error = %v, want invalid_argument", err)
			}
			if store.addCalls != 0 {
				t.Errorf("store Add() calls = %d, want 0", store.addCalls)
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
	if store.listCalls != 1 || store.listOptions != options {
		t.Errorf("store List() calls/options = %d/%#v, want 1/%#v", store.listCalls, store.listOptions, options)
	}

	invalidAreaID := int64(0)
	store = &recordingStore{}
	_, err := NewService(store).List(context.Background(), ListOptions{Status: ListStatusOpen, AreaID: &invalidAreaID})
	if errorCode(err) != apperr.InvalidArgument {
		t.Errorf("List() error = %v, want invalid_argument", err)
	}
	if store.listCalls != 0 {
		t.Errorf("store List() calls = %d, want 0", store.listCalls)
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
		edited.UpdatedAt != store.editTimestamp {
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
	invalid := []EditFields{
		{Area: AreaChange{Set: &invalidAreaID}},
		{Area: AreaChange{Set: &areaID, Clear: true}},
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

			resolvedProject := Project{ID: 7, Status: test.value}
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
			if len(transactionStore.lifecycleSequence) != 2 ||
				transactionStore.lifecycleSequence[0] != "resolve" ||
				transactionStore.lifecycleSequence[1] != "cancel open tasks" {
				t.Errorf(
					"transaction mutation sequence = %v, want resolve then cancel open tasks",
					transactionStore.lifecycleSequence,
				)
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
			if resolution.Project != resolvedProject {
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

	store := &recordingStore{reopenResult: Project{ID: 7, Status: string(ListStatusOpen)}}
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
	if reopened != store.reopenResult || store.reopenCalls != 1 || store.reopenID != 7 {
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

func TestDeleteUsesATransactionOnlyForRecursiveDeletion(t *testing.T) {
	t.Parallel()

	t.Run("nonrecursive", func(t *testing.T) {
		t.Parallel()

		deletedProject := Project{ID: 7, Title: "empty"}
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
		if deletion.Project != deletedProject || deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
			t.Errorf("Delete() = %#v, want project and non-nil empty deleted tasks", deletion)
		}
	})

	t.Run("recursive", func(t *testing.T) {
		t.Parallel()

		deletedProject := Project{ID: 7, Title: "populated"}
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
		if len(transactionStore.lifecycleSequence) != 2 ||
			transactionStore.lifecycleSequence[0] != "delete tasks" ||
			transactionStore.lifecycleSequence[1] != "delete project" {
			t.Errorf(
				"transaction mutation sequence = %v, want delete tasks then delete project",
				transactionStore.lifecycleSequence,
			)
		}
		if transactionStore.deleteTasksProjectID != 7 || transactionStore.deleteID != 7 {
			t.Errorf(
				"task/project delete IDs = %d/%d, want 7/7",
				transactionStore.deleteTasksProjectID,
				transactionStore.deleteID,
			)
		}
		if deletion.Project != deletedProject || len(deletion.DeletedTasks) != len(deletedTasks) ||
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

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
