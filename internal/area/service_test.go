package area

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type recordingStore struct {
	addCalls      int
	addFields     AddFields
	addTimestamp  string
	addResult     Area
	addError      error
	findCalls     int
	findID        int64
	findResult    Area
	findError     error
	listCalls     int
	listOptions   ListOptions
	listResult    []Area
	listError     error
	editCalls     int
	editID        int64
	editFields    EditFields
	editTimestamp string
	editResult    Area
	editError     error
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
			if store.addCalls+store.findCalls+store.listCalls+store.editCalls != 0 {
				t.Errorf("store received a call after invalid input: %#v", store)
			}
		})
	}
}

func TestParseIDAndListSliceAreStrict(t *testing.T) {
	t.Parallel()

	if id, err := ParseID("001"); err != nil || id != 1 {
		t.Fatalf("ParseID(001) = %d, %v, want 1, nil", id, err)
	}
	for _, value := range []string{"", "0", "-1", "+1", "1.0", "１２", "9223372036854775808"} {
		if _, err := ParseID(value); errorCode(err) != apperr.InvalidArgument {
			t.Errorf("ParseID(%q) error = %v, want invalid_argument", value, err)
		}
	}
	for _, value := range []ListSlice{ListSliceActive, ListSliceArchived, ListSliceAll} {
		if parsed, err := ParseListSlice(string(value)); err != nil || parsed != value {
			t.Errorf("ParseListSlice(%q) = %q, %v, want %q, nil", value, parsed, err, value)
		}
	}
	if _, err := ParseListSlice(""); errorCode(err) != apperr.InvalidArgument {
		t.Errorf("ParseListSlice(empty) error = %v, want invalid_argument", err)
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

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
