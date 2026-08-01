package store

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/logbook"
)

func TestLogbookListScansEntriesAndOrdersByResolutionKindAndID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (id, title, done_at, position)
VALUES (1, 'Alpha', '2026-02-01T00:00:00.000Z', 0);
INSERT INTO projects (id, title, cancelled_at, position)
VALUES (2, 'Beta', '2026-02-01T00:00:00.000Z', 1);
INSERT INTO tasks (id, title, done_at, position)
VALUES (1, 'first task', '2026-02-01T00:00:00.000Z', 0);
INSERT INTO tasks (id, project_id, title, cancelled_at, position)
VALUES (2, 1, 'second task', '2026-02-01T00:00:00.000Z', 1);
INSERT INTO tasks (id, project_id, title, done_at, position)
VALUES (3, 2, 'newest task', '2026-03-01T00:00:00.000Z', 2);
`); err != nil {
		t.Fatalf("seed logbook entries: %v", err)
	}

	entries, err := NewLogbook(storage).List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	alpha := "Alpha"
	beta := "Beta"
	want := []logbook.Entry{
		{
			Kind:         "task",
			ID:           3,
			Title:        "newest task",
			Status:       "done",
			ResolvedAt:   "2026-03-01T00:00:00.000Z",
			ProjectTitle: &beta,
		},
		{
			Kind:       "project",
			ID:         2,
			Title:      "Beta",
			Status:     "cancelled",
			ResolvedAt: "2026-02-01T00:00:00.000Z",
		},
		{
			Kind:       "project",
			ID:         1,
			Title:      "Alpha",
			Status:     "done",
			ResolvedAt: "2026-02-01T00:00:00.000Z",
		},
		{
			Kind:         "task",
			ID:           2,
			Title:        "second task",
			Status:       "cancelled",
			ResolvedAt:   "2026-02-01T00:00:00.000Z",
			ProjectTitle: &alpha,
		},
		{
			Kind:       "task",
			ID:         1,
			Title:      "first task",
			Status:     "done",
			ResolvedAt: "2026-02-01T00:00:00.000Z",
		},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("List() = %#v, want %#v", entries, want)
	}
}
