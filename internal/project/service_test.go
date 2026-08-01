package project

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type recordingStore struct {
	addCalls      int
	addFields     AddFields
	addTimestamp  string
	addError      error
	findCalls     int
	findID        int64
	findError     error
	listCalls     int
	listOptions   ListOptions
	listResult    []Project
	listError     error
	editCalls     int
	editID        int64
	editFields    EditFields
	editTimestamp string
	editError     error
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

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
