package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
)

type recordingStore struct {
	calls      int
	ctx        context.Context
	expression string
	result     []Hit
	err        error
}

func (s *recordingStore) Search(ctx context.Context, expression string) ([]Hit, error) {
	s.calls++
	s.ctx = ctx
	s.expression = expression
	return s.result, s.err
}

func TestServiceSearchRejectsInvalidExpressionBeforeStoreCall(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		expression string
		message    string
	}{
		{name: "empty", expression: "", message: "expression must not be blank"},
		{name: "whitespace", expression: " \t\n", message: "expression must not be blank"},
		{name: "invalid UTF-8", expression: string([]byte{0xff}), message: "expression must be valid UTF-8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			store := &recordingStore{}
			hits, err := NewService(store).Search(context.Background(), test.expression)
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("Search() error = %v, want invalid_argument", err)
			}
			if err == nil || err.Error() != test.message {
				t.Errorf("Search() error message = %v, want %q", err, test.message)
			}
			if hits != nil {
				t.Errorf("Search() hits = %#v, want nil", hits)
			}
			if store.calls != 0 {
				t.Errorf("store Search() calls = %d, want 0", store.calls)
			}
		})
	}
}

func TestServiceSearchDelegatesExpressionAndContext(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), contextKey{}, "request")
	want := []Hit{{Kind: KindTask, Task: &task.Task{ID: 7, Title: "Call plumber"}}}
	store := &recordingStore{result: want}

	got, err := NewService(store).Search(ctx, `plumb* OR "pipe wrench"`)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if store.calls != 1 || store.ctx != ctx || store.expression != `plumb* OR "pipe wrench"` {
		t.Errorf(
			"store Search() calls/context/expression = %d/%v/%q, want one call with supplied values",
			store.calls,
			store.ctx,
			store.expression,
		)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Search() = %#v, want %#v", got, want)
	}
}

func TestServiceSearchNormalizesNilHits(t *testing.T) {
	t.Parallel()

	hits, err := NewService(&recordingStore{}).Search(context.Background(), "missing")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if hits == nil || len(hits) != 0 {
		t.Errorf("Search() = %#v, want non-nil empty hits", hits)
	}
}

func TestServiceSearchReturnsStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("search failed")
	hits, err := NewService(&recordingStore{
		result: []Hit{{Kind: KindTask, Task: &task.Task{ID: 1}}},
		err:    storeErr,
	}).Search(context.Background(), "plumber")
	if !errors.Is(err, storeErr) {
		t.Errorf("Search() error = %v, want %v", err, storeErr)
	}
	if hits != nil {
		t.Errorf("Search() hits = %#v, want nil on error", hits)
	}
}

type contextKey struct{}
