package tag

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type recordingStore struct {
	addCalls         int
	addName          string
	addTimestamp     string
	addResult        Tag
	addError         error
	findCalls        int
	findName         string
	findResult       Tag
	findError        error
	listCalls        int
	listResult       []ListedTag
	listError        error
	renameCalls      int
	renameOldName    string
	renameNewName    string
	renameTimestamp  string
	renameResult     Tag
	renameError      error
	countUsageCalls  int
	countUsageName   string
	countUsageResult int64
	countUsageError  error
	deleteCalls      int
	deleteName       string
	deleteResult     Tag
	deleteError      error
	transactionCalls int
	transactionStore Store
	transactionError error
	sequence         []string
}

func (r *recordingStore) Add(
	_ context.Context,
	name string,
	timestamp string,
) (Tag, error) {
	r.addCalls++
	r.addName = name
	r.addTimestamp = timestamp
	return r.addResult, r.addError
}

func (r *recordingStore) Find(_ context.Context, name string) (Tag, error) {
	r.findCalls++
	r.findName = name
	r.sequence = append(r.sequence, "find")
	return r.findResult, r.findError
}

func (r *recordingStore) List(context.Context) ([]ListedTag, error) {
	r.listCalls++
	return r.listResult, r.listError
}

func (r *recordingStore) Rename(
	_ context.Context,
	oldName string,
	newName string,
	timestamp string,
) (Tag, error) {
	r.renameCalls++
	r.renameOldName = oldName
	r.renameNewName = newName
	r.renameTimestamp = timestamp
	r.sequence = append(r.sequence, "rename")
	return r.renameResult, r.renameError
}

func (r *recordingStore) CountUsage(_ context.Context, name string) (int64, error) {
	r.countUsageCalls++
	r.countUsageName = name
	r.sequence = append(r.sequence, "count usage")
	return r.countUsageResult, r.countUsageError
}

func (r *recordingStore) Delete(_ context.Context, name string) (Tag, error) {
	r.deleteCalls++
	r.deleteName = name
	r.sequence = append(r.sequence, "delete")
	return r.deleteResult, r.deleteError
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

func TestAddAndRenamePreserveNamesAndUseUTCMillisecondTimestamps(t *testing.T) {
	t.Parallel()

	addResult := Tag{ID: 1, Title: "  Errands  "}
	renameResult := Tag{ID: 1, Title: "  Out and About  "}
	store := &recordingStore{
		addResult:    addResult,
		findResult:   Tag{ID: 1, Title: "  Errands  "},
		renameResult: renameResult,
	}
	service := NewService(store)
	clockCalls := 0
	service.now = func() time.Time {
		clockCalls++
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

	added, err := service.Add(context.Background(), "  Errands  ")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	renamed, err := service.Rename(context.Background(), "  Errands  ", "  Out and About  ")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	wantRenaming := Renaming{PreviousTitle: "  Errands  ", Tag: renameResult}
	if added != addResult || renamed != wantRenaming {
		t.Errorf("Add()/Rename() = %#v/%#v, want %#v/%#v", added, renamed, addResult, wantRenaming)
	}
	if store.addCalls != 1 || store.addName != "  Errands  " || store.renameCalls != 1 ||
		store.renameOldName != "  Errands  " || store.renameNewName != "  Out and About  " {
		t.Errorf("delegated names/calls = %#v, want exact accepted text once", store)
	}
	const wantTimestamp = "2026-07-27T16:34:56.987Z"
	if clockCalls != 2 || store.addTimestamp != wantTimestamp ||
		store.renameTimestamp != wantTimestamp {
		t.Errorf(
			"clock/timestamps = %d/%q/%q, want 2/%q/%q",
			clockCalls,
			store.addTimestamp,
			store.renameTimestamp,
			wantTimestamp,
			wantTimestamp,
		)
	}
}

func TestRenameReturnsStoredPreviousSpellingFromOneTransaction(t *testing.T) {
	t.Parallel()

	previous := Tag{ID: 7, Title: "Errands", CreatedAt: "created", UpdatedAt: "before"}
	renamed := Tag{ID: 7, Title: "out-and-about", CreatedAt: "created", UpdatedAt: "after"}
	transactionStore := &recordingStore{findResult: previous, renameResult: renamed}
	store := &recordingStore{transactionStore: transactionStore}
	service := NewService(store)
	service.now = func() time.Time {
		return time.Date(2026, time.July, 27, 12, 34, 56, 0, time.UTC)
	}

	result, err := service.Rename(context.Background(), "ERRANDS", "out-and-about")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if result != (Renaming{PreviousTitle: previous.Title, Tag: renamed}) {
		t.Errorf("Rename() = %#v, want stored previous spelling and renamed tag", result)
	}
	if store.transactionCalls != 1 || store.findCalls+store.renameCalls != 0 {
		t.Errorf("outer calls = %#v, want one transaction boundary only", store)
	}
	if !reflect.DeepEqual(transactionStore.sequence, []string{"find", "rename"}) {
		t.Errorf("transaction sequence = %v, want find then rename", transactionStore.sequence)
	}
	if transactionStore.findName != "ERRANDS" || transactionStore.renameOldName != "ERRANDS" ||
		transactionStore.renameNewName != "out-and-about" {
		t.Errorf("transaction names = %#v, want caller identities delegated exactly", transactionStore)
	}
}

func TestMutationsRejectInvalidNamesBeforePersistence(t *testing.T) {
	t.Parallel()

	blank := " \t\n"
	invalidUTF8 := string([]byte{0xff})
	tests := []struct {
		name  string
		apply func(*Service) error
	}{
		{name: "add blank", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), blank)
			return err
		}},
		{name: "add invalid UTF-8", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), invalidUTF8)
			return err
		}},
		{name: "rename blank source", apply: func(service *Service) error {
			_, err := service.Rename(context.Background(), blank, "valid")
			return err
		}},
		{name: "rename invalid target UTF-8", apply: func(service *Service) error {
			_, err := service.Rename(context.Background(), "valid", invalidUTF8)
			return err
		}},
		{name: "delete blank", apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), blank)
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			service := NewService(store)
			clockCalls := 0
			service.now = func() time.Time {
				clockCalls++
				return time.Time{}
			}

			if err := test.apply(service); errorCode(err) != apperr.InvalidArgument {
				t.Errorf("operation error = %v, want invalid_argument", err)
			}
			if store.addCalls+store.renameCalls+store.transactionCalls != 0 || clockCalls != 0 {
				t.Errorf("store/clock received calls after invalid input: %#v/%d", store, clockCalls)
			}
		})
	}
}

func TestListNormalizesNilAndPreservesStoreResults(t *testing.T) {
	t.Parallel()

	store := &recordingStore{}
	listed, err := NewService(store).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listed == nil || len(listed) != 0 || store.listCalls != 1 {
		t.Errorf("List()/calls = %#v/%d, want non-nil empty list/1", listed, store.listCalls)
	}

	want := []ListedTag{{Tag: Tag{ID: 1, Title: "errands"}, UsageCount: 3}}
	store.listResult = want
	listed, err = NewService(store).List(context.Background())
	if err != nil {
		t.Fatalf("List(nonempty) error = %v", err)
	}
	if !reflect.DeepEqual(listed, want) {
		t.Errorf("List(nonempty) = %#v, want %#v", listed, want)
	}
}

func TestDeleteOwnsFindCountDeleteTransactionAndReturnsDeletedTag(t *testing.T) {
	t.Parallel()

	found := Tag{ID: 7, Title: "errands", CreatedAt: "created", UpdatedAt: "updated"}
	deleted := Tag{ID: 7, Title: "errands", CreatedAt: "created", UpdatedAt: "updated"}
	transactionStore := &recordingStore{
		findResult:       found,
		countUsageResult: 3,
		deleteResult:     deleted,
	}
	store := &recordingStore{transactionStore: transactionStore}

	deletion, err := NewService(store).Delete(context.Background(), "ERRANDS")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.transactionCalls != 1 || store.findCalls+store.countUsageCalls+store.deleteCalls != 0 {
		t.Errorf("outer calls = %#v, want one transaction boundary only", store)
	}
	if !reflect.DeepEqual(transactionStore.sequence, []string{"find", "count usage", "delete"}) {
		t.Errorf("transaction sequence = %v, want find, count usage, delete", transactionStore.sequence)
	}
	if transactionStore.findName != "ERRANDS" || transactionStore.countUsageName != "ERRANDS" ||
		transactionStore.deleteName != "ERRANDS" {
		t.Errorf("transaction names = %#v, want caller identity delegated exactly", transactionStore)
	}
	if deletion.Tag != deleted || deletion.Detached != 3 {
		t.Errorf("Delete() = %#v, want deleted tag %#v and detached count 3", deletion, deleted)
	}
}

func TestStoreAndTransactionErrorsPassThroughUnchanged(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.Conflict, "store conflict", nil)
	tests := []struct {
		name  string
		store *recordingStore
		apply func(*Service) error
	}{
		{name: "add", store: &recordingStore{addError: storeError}, apply: func(service *Service) error {
			_, err := service.Add(context.Background(), "valid")
			return err
		}},
		{name: "list", store: &recordingStore{listError: storeError}, apply: func(service *Service) error {
			_, err := service.List(context.Background())
			return err
		}},
		{name: "rename", store: &recordingStore{renameError: storeError}, apply: func(service *Service) error {
			_, err := service.Rename(context.Background(), "old", "new")
			return err
		}},
		{name: "transaction boundary", store: &recordingStore{transactionError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), "valid")
			return err
		}},
		{name: "delete find", store: &recordingStore{findError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), "valid")
			return err
		}},
		{name: "delete usage count", store: &recordingStore{countUsageError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), "valid")
			return err
		}},
		{name: "delete mutation", store: &recordingStore{deleteError: storeError}, apply: func(service *Service) error {
			_, err := service.Delete(context.Background(), "valid")
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
