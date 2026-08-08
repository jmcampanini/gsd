package area

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
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
	findResults          []Area
	findErrors           []error
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
	reorderCalls         int
	reorderID            int64
	reorderPlacement     domain.Placement
	reorderTimestamp     string
	reorderResult        Area
	reorderError         error
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
	occupiedCalls        int
	occupiedID           int64
	occupied             bool
	occupiedError        error
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
	resolveTagsCalls     int
	resolveTagNames      [][]string
	resolveTagsResult    []tag.Tag
	resolveTagsError     error
	attachTagsCalls      int
	attachTagsAreaID     int64
	attachedTags         []tag.Tag
	attachTagsError      error
	detachTagsCalls      int
	detachTagsAreaID     int64
	detachedTags         []tag.Tag
	detachTagsError      error
	transactionCalls     int
	transactionStore     Transaction
	transactionError     error
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
	index := r.findCalls - 1
	if index < len(r.findErrors) && r.findErrors[index] != nil {
		return Area{}, r.findErrors[index]
	}
	if index < len(r.findResults) {
		return r.findResults[index], nil
	}
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

func (r *recordingStore) Reorder(
	_ context.Context,
	id int64,
	placement domain.Placement,
	timestamp string,
) (Area, error) {
	r.reorderCalls++
	r.reorderID = id
	r.reorderPlacement = placement
	r.reorderTimestamp = timestamp
	return r.reorderResult, r.reorderError
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

func (r *recordingStore) Occupied(_ context.Context, id int64) (bool, error) {
	r.occupiedCalls++
	r.occupiedID = id
	return r.occupied, r.occupiedError
}

func (r *recordingStore) Delete(_ context.Context, id int64) (Area, error) {
	r.deleteCalls++
	r.deleteID = id
	return r.deleteResult, r.deleteError
}

func (r *recordingStore) DeleteProjects(
	_ context.Context,
	areaID int64,
) ([]project.Project, error) {
	r.deleteProjectsCalls++
	r.deleteProjectsAreaID = areaID
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
	if scope == TaskDeletionScopeProject {
		return r.projectTasksResult, nil
	}
	return r.looseTasksResult, nil
}

func (r *recordingStore) ResolveTags(_ context.Context, names []string) ([]tag.Tag, error) {
	r.resolveTagsCalls++
	r.resolveTagNames = append(r.resolveTagNames, append([]string(nil), names...))
	return r.resolveTagsResult, r.resolveTagsError
}

func (r *recordingStore) AttachTags(_ context.Context, areaID int64, tags []tag.Tag) error {
	r.attachTagsCalls++
	r.attachTagsAreaID = areaID
	r.attachedTags = append([]tag.Tag(nil), tags...)
	return r.attachTagsError
}

func (r *recordingStore) DetachTags(_ context.Context, areaID int64, tags []tag.Tag) error {
	r.detachTagsCalls++
	r.detachTagsAreaID = areaID
	r.detachedTags = append([]tag.Tag(nil), tags...)
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

func TestAddPreservesAcceptedTextAndUsesOneUTCMillisecondTimestamp(t *testing.T) {
	t.Parallel()

	fields := AddFields{Title: "  Home  ", Note: "line one\nline two\n"}
	want := Area{ID: 1, Title: fields.Title, Note: fields.Note, Tags: domain.TagNames{}}
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
	if !reflect.DeepEqual(got, want) || store.addCalls != 1 || !equalAddFields(store.addFields, fields) {
		t.Errorf("Add() result/calls/fields = %#v/%d/%#v, want %#v/1/%#v", got, store.addCalls, store.addFields, want, fields)
	}
	if store.transactionCalls != 0 {
		t.Errorf("transaction calls = %d, want direct add for no tags", store.transactionCalls)
	}
	if clockCalls != 1 || store.addTimestamp != "2026-07-27T16:34:56.987Z" {
		t.Errorf("clock calls/timestamp = %d/%q, want 1/UTC milliseconds", clockCalls, store.addTimestamp)
	}
}

func TestAddWithTagsNormalizesAndRefreshesWithinOneTransaction(t *testing.T) {
	t.Parallel()

	fields := AddFields{
		Title: "Home",
		Tags:  []string{"Errands", "ERRANDS", "Café", "CAFÉ"},
	}
	normalizedNames := []string{"Errands", "Café", "CAFÉ"}
	resolvedTags := []tag.Tag{
		{ID: 2, Title: "errands"},
		{ID: 3, Title: "Café"},
		{ID: 4, Title: "CAFÉ"},
	}
	refreshed := Area{ID: 7, Title: "Home", Tags: domain.TagNames{"errands", "Café", "CAFÉ"}}
	transactionStore := &recordingStore{
		addResult:         Area{ID: 7, Title: "Home"},
		resolveTagsResult: resolvedTags,
		findResult:        refreshed,
	}
	store := &recordingStore{transactionStore: transactionStore}
	service := NewService(store)
	service.now = func() time.Time {
		return time.Date(2026, time.July, 27, 12, 34, 56, 987654321, time.UTC)
	}

	got, err := service.Add(context.Background(), fields)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !reflect.DeepEqual(got, refreshed) {
		t.Errorf("Add() = %#v, want refreshed %#v", got, refreshed)
	}
	if store.transactionCalls != 1 || store.addCalls+store.resolveTagsCalls+
		store.attachTagsCalls+store.findCalls != 0 {
		t.Errorf("outer store calls = %#v, want transaction boundary only", store)
	}
	wantFields := fields
	wantFields.Tags = normalizedNames
	if !reflect.DeepEqual(transactionStore.addFields, wantFields) ||
		transactionStore.addTimestamp != "2026-07-27T12:34:56.987Z" {
		t.Errorf("transaction add fields/timestamp = %#v/%q, want %#v/UTC milliseconds", transactionStore.addFields, transactionStore.addTimestamp, wantFields)
	}
	if !reflect.DeepEqual(transactionStore.resolveTagNames, [][]string{normalizedNames}) ||
		transactionStore.attachTagsAreaID != 7 ||
		!reflect.DeepEqual(transactionStore.attachedTags, resolvedTags) {
		t.Errorf("tag resolution/attachment = %#v, want normalized names and resolved tags on area 7", transactionStore)
	}
	if transactionStore.addCalls != 1 || transactionStore.resolveTagsCalls != 1 ||
		transactionStore.attachTagsCalls != 1 || transactionStore.findCalls != 1 {
		t.Errorf("transaction-scoped calls = %#v, want one add, resolve, attach, and refresh", transactionStore)
	}
}

func TestTaggedAddErrorsPassThrough(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.NotFound, "missing", nil)
	resolvedTags := []tag.Tag{{ID: 2, Title: "home"}}
	tests := []struct {
		name        string
		outer       *recordingStore
		transaction *recordingStore
	}{
		{
			name:  "transaction boundary",
			outer: &recordingStore{transactionError: storeError},
		},
		{
			name:        "area add",
			transaction: &recordingStore{addError: storeError},
		},
		{
			name: "tag resolution",
			transaction: &recordingStore{
				addResult: Area{ID: 7}, resolveTagsError: storeError,
			},
		},
		{
			name: "tag attachment",
			transaction: &recordingStore{
				addResult: Area{ID: 7}, resolveTagsResult: resolvedTags, attachTagsError: storeError,
			},
		},
		{
			name: "refreshed lookup",
			transaction: &recordingStore{
				addResult: Area{ID: 7}, resolveTagsResult: resolvedTags, findError: storeError,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outer := test.outer
			if outer == nil {
				outer = &recordingStore{transactionStore: test.transaction}
			}
			_, err := NewService(outer).Add(
				context.Background(),
				AddFields{Title: "Home", Tags: []string{"home"}},
			)
			if !errors.Is(err, storeError) {
				t.Errorf("Add() error = %v, want preserved %v", err, storeError)
			}
			if outer.transactionCalls != 1 {
				t.Errorf("transaction calls = %d, want 1", outer.transactionCalls)
			}
		})
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
		{name: "blank add tag", apply: func(service *Service) error {
			_, err := service.Add(context.Background(), AddFields{Title: validTitle, Tags: []string{" "}})
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
		{name: "nonpositive tag ID", apply: func(service *Service) error {
			_, err := service.Tag(context.Background(), 0, []string{"home"})
			return err
		}},
		{name: "tag without names", apply: func(service *Service) error {
			_, err := service.Tag(context.Background(), 1, nil)
			return err
		}},
		{name: "untag without names", apply: func(service *Service) error {
			_, err := service.Untag(context.Background(), 1, []string{})
			return err
		}},
		{name: "blank tag name", apply: func(service *Service) error {
			_, err := service.Tag(context.Background(), 1, []string{"home", "\t"})
			return err
		}},
		{name: "invalid untag name UTF-8", apply: func(service *Service) error {
			_, err := service.Untag(context.Background(), 1, []string{invalidText})
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
				store.archiveCalls+store.unarchiveCalls+store.deleteCalls+store.resolveTagsCalls+
				store.attachTagsCalls+store.detachTagsCalls+store.transactionCalls != 0 {
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

	placement := domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 11}
	want := Area{ID: 7, Title: "moved", Position: 2}
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

func TestArchiveAndUnarchiveDelegateTimestamps(t *testing.T) {
	t.Parallel()

	want := Area{ID: 7, Title: "Home", Tags: domain.TagNames{}}
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
	if !reflect.DeepEqual(archived, want) || !reflect.DeepEqual(unarchived, want) || store.archiveCalls != 1 ||
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

func TestTagAndUntagNormalizeWithinOneTransactionAndReturnStoredSpellings(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-07-27T12:00:00.000Z"
	found := Area{ID: 7, Title: "Archived", ArchivedAt: &archivedAt}
	resolvedTags := []tag.Tag{{ID: 2, Title: "Errands"}, {ID: 3, Title: "CAFÉ"}}
	names := []string{"ERRANDS", "errands", "CAFÉ"}
	normalizedNames := []string{"ERRANDS", "CAFÉ"}
	tests := []struct {
		name         string
		apply        func(*Service) (Tagging, error)
		mutationName string
	}{
		{
			name: "tag",
			apply: func(service *Service) (Tagging, error) {
				return service.Tag(context.Background(), 7, names)
			},
			mutationName: "attach tags",
		},
		{
			name: "untag",
			apply: func(service *Service) (Tagging, error) {
				return service.Untag(context.Background(), 7, names)
			},
			mutationName: "detach tags",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			refreshed := found
			if test.mutationName == "attach tags" {
				refreshed.Tags = domain.TagNames{"Errands", "CAFÉ"}
			} else {
				refreshed.Tags = domain.TagNames{}
			}
			transactionStore := &recordingStore{
				findResults:       []Area{found, refreshed},
				resolveTagsResult: resolvedTags,
			}
			store := &recordingStore{transactionStore: transactionStore}
			service := NewService(store)
			service.now = func() time.Time {
				t.Fatal("tagging must not request a timestamp")
				return time.Time{}
			}

			got, err := test.apply(service)
			if err != nil {
				t.Fatalf("operation error = %v", err)
			}
			want := Tagging{Area: refreshed, TagTitles: []string{"Errands", "CAFÉ"}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("operation result = %#v, want refreshed area and stored spellings %#v", got, want)
			}
			if store.transactionCalls != 1 || store.findCalls+store.resolveTagsCalls+
				store.attachTagsCalls+store.detachTagsCalls != 0 {
				t.Errorf("outer store calls = %#v, want transaction boundary only", store)
			}
			if transactionStore.findCalls != 2 || transactionStore.findID != 7 ||
				transactionStore.resolveTagsCalls != 1 ||
				!reflect.DeepEqual(transactionStore.resolveTagNames, [][]string{normalizedNames}) {
				t.Errorf("find/resolution intent = %#v, want area 7 and normalized names", transactionStore)
			}
			if test.mutationName == "attach tags" {
				if transactionStore.attachTagsCalls != 1 || transactionStore.attachTagsAreaID != 7 ||
					!reflect.DeepEqual(transactionStore.attachedTags, resolvedTags) || transactionStore.detachTagsCalls != 0 {
					t.Errorf("attachment intent = %#v, want resolved tags attached once", transactionStore)
				}
			} else if transactionStore.detachTagsCalls != 1 || transactionStore.detachTagsAreaID != 7 ||
				!reflect.DeepEqual(transactionStore.detachedTags, resolvedTags) || transactionStore.attachTagsCalls != 0 {
				t.Errorf("detachment intent = %#v, want resolved tags detached once", transactionStore)
			}
		})
	}
}

func TestTaggingErrorsPassThrough(t *testing.T) {
	t.Parallel()

	storeError := apperr.New(apperr.NotFound, "missing", nil)
	tests := []struct {
		name        string
		outer       *recordingStore
		transaction *recordingStore
		apply       func(*Service) error
	}{
		{
			name:  "transaction boundary",
			outer: &recordingStore{transactionError: storeError},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"home"})
				return err
			},
		},
		{
			name:        "entity lookup",
			transaction: &recordingStore{findError: storeError},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"home"})
				return err
			},
		},
		{
			name:        "tag resolution",
			transaction: &recordingStore{findResult: Area{ID: 7}, resolveTagsError: storeError},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"home"})
				return err
			},
		},
		{
			name: "attachment",
			transaction: &recordingStore{
				findResult: Area{ID: 7}, resolveTagsResult: []tag.Tag{{ID: 2, Title: "home"}}, attachTagsError: storeError,
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"home"})
				return err
			},
		},
		{
			name: "detachment",
			transaction: &recordingStore{
				findResult: Area{ID: 7}, resolveTagsResult: []tag.Tag{{ID: 2, Title: "home"}}, detachTagsError: storeError,
			},
			apply: func(service *Service) error {
				_, err := service.Untag(context.Background(), 7, []string{"home"})
				return err
			},
		},
		{
			name: "refreshed lookup",
			transaction: &recordingStore{
				findResults: []Area{{ID: 7}}, findErrors: []error{nil, storeError},
				resolveTagsResult: []tag.Tag{{ID: 2, Title: "home"}},
			},
			apply: func(service *Service) error {
				_, err := service.Tag(context.Background(), 7, []string{"home"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outer := test.outer
			if outer == nil {
				outer = &recordingStore{transactionStore: test.transaction}
			}
			if err := test.apply(NewService(outer)); !errors.Is(err, storeError) {
				t.Errorf("operation error = %v, want preserved %v", err, storeError)
			}
			if outer.transactionCalls != 1 {
				t.Errorf("transaction calls = %d, want 1", outer.transactionCalls)
			}
		})
	}
}

func TestNonrecursiveDeleteChecksOccupancyBeforeDeletingInOneTransaction(t *testing.T) {
	t.Parallel()

	deletedArea := Area{ID: 7, Title: "Empty", Tags: domain.TagNames{}}
	store := &recordingStore{deleteResult: deletedArea}
	deletion, err := NewService(store).Delete(context.Background(), 7, false)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.transactionCalls != 1 || store.occupiedCalls != 1 || store.occupiedID != 7 ||
		store.deleteCalls != 1 || store.deleteID != 7 ||
		store.deleteProjectsCalls != 0 || store.deleteTasksCalls != 0 {
		t.Errorf("delete delegation = %#v, want occupancy check then delete in one transaction", store)
	}
	if !reflect.DeepEqual(deletion.Area, deletedArea) || deletion.DeletedProjects == nil ||
		len(deletion.DeletedProjects) != 0 || deletion.DeletedTasks == nil || len(deletion.DeletedTasks) != 0 {
		t.Errorf("Delete() = %#v, want area and non-nil empty arrays", deletion)
	}
}

func TestNonrecursiveDeleteRefusesOccupiedAreaWithoutDeleting(t *testing.T) {
	t.Parallel()

	store := &recordingStore{occupied: true}
	_, err := NewService(store).Delete(context.Background(), 7, false)
	if code, _ := apperr.CodeOf(err); code != apperr.Conflict ||
		!strings.Contains(err.Error(), "cannot delete area 7 while it contains projects or tasks") {
		t.Fatalf("Delete() error = %v, want occupied conflict", err)
	}
	if store.deleteCalls != 0 {
		t.Errorf("Delete() store deletes = %d, want 0", store.deleteCalls)
	}
}

func TestRecursiveDeleteReturnsContainerGroupedEnvelope(t *testing.T) {
	t.Parallel()

	deletedArea := Area{ID: 7, Title: "Home", Tags: domain.TagNames{}}
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
	if transactionStore.deleteProjectsCalls != 1 || transactionStore.deleteTasksCalls != 2 ||
		transactionStore.deleteCalls != 1 {
		t.Errorf("transaction-scoped calls = %#v, want one project, two task, and one area deletion", transactionStore)
	}
	if transactionStore.deleteProjectsAreaID != 7 ||
		!reflect.DeepEqual(transactionStore.deleteTasksAreaIDs, []int64{7, 7}) ||
		!reflect.DeepEqual(
			transactionStore.deleteTasksScopes,
			[]TaskDeletionScope{TaskDeletionScopeProject, TaskDeletionScopeLoose},
		) || transactionStore.deleteID != 7 {
		t.Errorf("recursive delete intent = %#v, want area 7 with project then loose scopes", transactionStore)
	}
	if !reflect.DeepEqual(deletion.Area, deletedArea) {
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

func equalAddFields(left, right AddFields) bool {
	return left.Title == right.Title && left.Note == right.Note && slices.Equal(left.Tags, right.Tags)
}

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}
