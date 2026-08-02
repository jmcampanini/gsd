package logbook

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingStore struct {
	calls  int
	ctx    context.Context
	result []Entry
	err    error
}

func (s *recordingStore) List(ctx context.Context) ([]Entry, error) {
	s.calls++
	s.ctx = ctx
	return s.result, s.err
}

func TestServiceListDelegatesAndReturnsEntries(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	want := []Entry{{Kind: "task", ID: 7, Title: "shipped", Status: "done", Tags: []string{}}}
	store := &recordingStore{result: []Entry{{Kind: "task", ID: 7, Title: "shipped", Status: "done", Tags: []string{}}}}

	got, err := NewService(store).List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if store.calls != 1 || store.ctx != ctx {
		t.Errorf("store List() calls/context = %d/%v, want one call with supplied context", store.calls, store.ctx)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %#v, want %#v", got, want)
	}
}

func TestServiceListNormalizesNilEntries(t *testing.T) {
	t.Parallel()

	entries, err := NewService(&recordingStore{}).List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if entries == nil || len(entries) != 0 {
		t.Errorf("List() = %#v, want non-nil empty entries", entries)
	}
}

func TestServiceListReturnsStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("list failed")
	entries, err := NewService(&recordingStore{result: []Entry{{ID: 1}}, err: storeErr}).List(context.Background())
	if !errors.Is(err, storeErr) {
		t.Errorf("List() error = %v, want %v", err, storeErr)
	}
	if entries != nil {
		t.Errorf("List() entries = %#v, want nil on error", entries)
	}
}

type contextKey struct{}
