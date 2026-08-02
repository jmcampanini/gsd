package task

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type recordingStore struct {
	addCalls           int
	title              string
	note               string
	addedProjectID     *int64
	addedAreaID        *int64
	addedDueOn         *string
	addedDeferUntil    *string
	timestamp          string
	inboxCalls         int
	inboxResult        []ViewTask
	availableCalls     int
	availableResult    []ViewTask
	findCalls          int
	listCalls          int
	listedOptions      ListOptions
	listResult         []Task
	editCalls          int
	editID             int64
	editFields         EditFields
	editTimestamp      string
	doneCalls          int
	cancelCalls        int
	reopenCalls        int
	deleteCalls        int
	lifecycleID        int64
	lifecycleTimestamp string
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
	r.timestamp = timestamp

	return Task{
		ID:         1,
		ProjectID:  fields.ProjectID,
		AreaID:     fields.AreaID,
		Title:      fields.Title,
		Note:       fields.Note,
		DueOn:      fields.DueOn,
		DeferUntil: fields.DeferUntil,
		CreatedAt:  timestamp,
		UpdatedAt:  timestamp,
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
	return Task{ID: id}, nil
}

func (r *recordingStore) List(_ context.Context, options ListOptions) ([]Task, error) {
	r.listCalls++
	r.listedOptions = options
	return r.listResult, nil
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

	return Task{ID: id, UpdatedAt: timestamp}, nil
}

func (r *recordingStore) Done(_ context.Context, id int64, timestamp string) (Task, error) {
	r.doneCalls++
	r.recordLifecycle(id, timestamp)
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

func (r *recordingStore) WithinTransaction(
	ctx context.Context,
	operation func(Store) error,
) error {
	return operation(r)
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
	created, err := service.Add(context.Background(), AddFields{
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
	if _, err := NewService(store).Add(context.Background(), AddFields{AreaID: &areaID, Title: "area task"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if store.addCalls != 1 || store.addedAreaID == nil || *store.addedAreaID != areaID {
		t.Errorf("store Add() calls/area = %d/%#v, want 1/%d", store.addCalls, store.addedAreaID, areaID)
	}

	projectID := int64(7)
	invalidAreaID := int64(0)
	for _, fields := range []AddFields{
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
	invalidProjectID := int64(0)
	tests := []struct {
		name       string
		projectID  *int64
		title      string
		note       string
		dueOn      *string
		deferUntil *string
	}{
		{name: "blank title", title: " \t\n"},
		{name: "invalid title UTF-8", title: string([]byte{0xff})},
		{name: "invalid note UTF-8", title: "valid", note: string([]byte{0xff})},
		{name: "nonpositive project ID", projectID: &invalidProjectID, title: "valid"},
		{name: "invalid due date", title: "valid", dueOn: &invalidDate},
		{name: "invalid defer date", title: "valid", deferUntil: &invalidDate},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			service := NewService(store)
			_, err := service.Add(context.Background(), AddFields{
				ProjectID:  test.projectID,
				Title:      test.title,
				Note:       test.note,
				DueOn:      test.dueOn,
				DeferUntil: test.deferUntil,
			})
			if err == nil {
				t.Fatal("Add() error = nil, want invalid_argument")
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.InvalidArgument {
				t.Errorf("Add() error = %v, want invalid_argument", err)
			}
			if store.addCalls != 0 {
				t.Errorf("store Add() calls = %d, want 0", store.addCalls)
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
	projectID := int64(7)
	options := ListOptions{
		Status:    ListStatusDone,
		Date:      DateSelectorDeferred,
		ProjectID: &projectID,
	}

	listed, err := service.List(context.Background(), options)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listed == nil || len(listed) != 0 {
		t.Errorf("List() = %#v, want non-nil empty list", listed)
	}
	if store.listCalls != 1 || store.listedOptions != options {
		t.Errorf("store List() calls/options = %d/%#v, want 1/%#v", store.listCalls, store.listedOptions, options)
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
	if store.listCalls != 1 || store.listedOptions != options {
		t.Errorf("store List() calls/options = %d/%#v, want 1/%#v", store.listCalls, store.listedOptions, options)
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
	edited, err := service.Edit(context.Background(), 7, EditFields{
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
	if nowCalls != 1 || store.editTimestamp != "2026-07-27T16:34:56.987Z" || edited.UpdatedAt != store.editTimestamp {
		t.Errorf("clock calls/timestamp = %d/%q, want one call and UTC milliseconds", nowCalls, store.editTimestamp)
	}
}

func TestEditDistinguishesClearedFieldsFromOmittedFields(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	note := ""
	_, err := NewService(store).Edit(context.Background(), 7, EditFields{
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
	_, err = NewService(omittedStore).Edit(context.Background(), 7, EditFields{Title: &title})
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

	projectID := int64(7)
	invalidAreaID := int64(0)
	invalid := []EditFields{
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
		fields EditFields
	}{
		{name: "nonpositive ID", fields: EditFields{Title: &validTitle}},
		{name: "no fields", id: 1},
		{name: "blank title", id: 1, fields: EditFields{Title: &blankTitle}},
		{name: "invalid title UTF-8", id: 1, fields: EditFields{Title: &invalidTitle}},
		{name: "invalid note UTF-8", id: 1, fields: EditFields{Note: &invalidNote}},
		{
			name: "nonpositive project ID",
			id:   1,
			fields: EditFields{Project: ProjectChange{
				Set: &invalidProjectID,
			}},
		},
		{
			name: "set and clear project",
			id:   1,
			fields: EditFields{Project: ProjectChange{
				Set:   &validProjectID,
				Clear: true,
			}},
		},
		{name: "invalid due date", id: 1, fields: EditFields{DueOn: DateChange{Set: &invalidDate}}},
		{name: "invalid defer date", id: 1, fields: EditFields{DeferUntil: DateChange{Set: &invalidDate}}},
		{
			name: "set and clear due date",
			id:   1,
			fields: EditFields{DueOn: DateChange{
				Set:   &validDate,
				Clear: true,
			}},
		},
		{
			name: "set and clear defer date",
			id:   1,
			fields: EditFields{DeferUntil: DateChange{
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
			if store.editCalls != 0 {
				t.Errorf("store Edit() calls = %d, want 0", store.editCalls)
			}
		})
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
