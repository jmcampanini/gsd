package store

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tag"
)

func TestTagsAddAndFindUseOneCaseInsensitiveNamespace(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tags := NewTags(storage)
	created, err := tags.Add(ctx, "Errands", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if created.ID <= 0 || created.Title != "Errands" ||
		created.CreatedAt != "2026-01-01T00:00:00.000Z" ||
		created.UpdatedAt != "2026-01-01T00:00:00.000Z" {
		t.Errorf("Add() = %#v, want stored spelling and supplied timestamps", created)
	}

	found, err := tags.Find(ctx, "eRrAnDs")
	if err != nil {
		t.Fatalf("Find(case variant) error = %v", err)
	}
	if !reflect.DeepEqual(found, created) {
		t.Errorf("Find(case variant) = %#v, want %#v", found, created)
	}

	if _, err := tags.Add(ctx, "ERRANDS", "2026-01-02T00:00:00.000Z"); errorCode(err) != apperr.Conflict ||
		!strings.Contains(err.Error(), "Errands") {
		t.Errorf("Add(case duplicate) error = %v, want conflict naming stored spelling", err)
	}
	if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM tags"); count != 1 {
		t.Errorf("tag count after conflict = %d, want 1", count)
	}

	lower, err := tags.Add(ctx, "é", "2026-01-03T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(non-ASCII lowercase) error = %v", err)
	}
	upper, err := tags.Add(ctx, "É", "2026-01-04T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(non-ASCII uppercase) error = %v", err)
	}
	if lower.ID == upper.ID || lower.Title != "é" || upper.Title != "É" {
		t.Errorf("non-ASCII tags = %#v and %#v, want distinct stored variants", lower, upper)
	}
	for name, want := range map[string]tag.Tag{"é": lower, "É": upper} {
		found, findErr := tags.Find(ctx, name)
		if findErr != nil {
			t.Errorf("Find(%q) error = %v", name, findErr)
		} else if !reflect.DeepEqual(found, want) {
			t.Errorf("Find(%q) = %#v, want %#v", name, found, want)
		}
	}
}

func TestTagsListAndCountUsageAggregateEveryEntityKindAndOrderNoCase(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tags := NewTags(storage)
	created := make(map[string]tag.Tag)
	for _, name := range []string{"zulu", "Alpha", "beta"} {
		value, err := tags.Add(ctx, name, "2026-01-01T00:00:00.000Z")
		if err != nil {
			t.Fatalf("Add(%s) error = %v", name, err)
		}
		created[name] = value
	}

	taskID := insertFixture(t, storage.database, "INSERT INTO tasks (title, position) VALUES ('task', 0)")
	projectID := insertFixture(t, storage.database, "INSERT INTO projects (title, position) VALUES ('project', 0)")
	areaID := insertFixture(t, storage.database, "INSERT INTO areas (title, position) VALUES ('area', 0)")
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{query: "INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)", args: []any{taskID, created["Alpha"].ID}},
		{query: "INSERT INTO project_tags (project_id, tag_id) VALUES (?, ?)", args: []any{projectID, created["Alpha"].ID}},
		{query: "INSERT INTO area_tags (area_id, tag_id) VALUES (?, ?)", args: []any{areaID, created["Alpha"].ID}},
		{query: "INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)", args: []any{taskID, created["beta"].ID}},
	} {
		if _, err := storage.database.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("insert usage fixture: %v", err)
		}
	}

	listed, err := tags.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []tag.ListedTag{
		{Tag: created["Alpha"], UsageCount: 3},
		{Tag: created["beta"], UsageCount: 1},
		{Tag: created["zulu"], UsageCount: 0},
	}
	if !reflect.DeepEqual(listed, want) {
		t.Errorf("List() = %#v, want NOCASE order and total usage %#v", listed, want)
	}
	usage, err := tags.CountUsage(ctx, "aLPHa")
	if err != nil || usage != 3 {
		t.Errorf("CountUsage(case variant) = %d, %v; want 3", usage, err)
	}
	if _, err := tags.CountUsage(ctx, "missing"); errorCode(err) != apperr.NotFound {
		t.Errorf("CountUsage(missing) error = %v, want not_found", err)
	}
}

func TestTagsRenameSupportsCaseOnlyChangesAndClassifiesFailures(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tags := NewTags(storage)
	errands, err := tags.Add(ctx, "Errands", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(Errands) error = %v", err)
	}
	home, err := tags.Add(ctx, "Home", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(Home) error = %v", err)
	}

	renamed, err := tags.Rename(ctx, "ERRANDS", "errands", "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Rename(case only) error = %v", err)
	}
	if renamed.ID != errands.ID || renamed.Title != "errands" ||
		renamed.CreatedAt != errands.CreatedAt || renamed.UpdatedAt != "2026-01-02T00:00:00.000Z" {
		t.Errorf("Rename(case only) = %#v, want same row with new spelling and timestamp", renamed)
	}

	if _, err := tags.Rename(ctx, "errands", "HOME", "2026-01-03T00:00:00.000Z"); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), home.Title) {
		t.Errorf("Rename(to existing) error = %v, want conflict naming %q", err, home.Title)
	}
	persisted, err := tags.Find(ctx, "ERRANDS")
	if err != nil || !reflect.DeepEqual(persisted, renamed) {
		t.Errorf("Find(after rename conflict) = %#v, %v; want unchanged %#v", persisted, err, renamed)
	}
	if _, err := tags.Rename(ctx, "ghost", "new", "2026-01-04T00:00:00.000Z"); errorCode(err) != apperr.NotFound || !strings.Contains(err.Error(), "ghost") {
		t.Errorf("Rename(missing) error = %v, want not_found naming source", err)
	}
}

func TestTagsDeleteMatchesCaseAndReturnsStoredSnapshot(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tags := NewTags(storage)
	created, err := tags.Add(ctx, "Errands", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	deleted, err := tags.Delete(ctx, "ERRANDS")
	if err != nil {
		t.Fatalf("Delete(case variant) error = %v", err)
	}
	if !reflect.DeepEqual(deleted, created) {
		t.Errorf("Delete() = %#v, want snapshot %#v", deleted, created)
	}
	if _, err := tags.Find(ctx, "Errands"); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(deleted) error = %v, want not_found", err)
	}
	if _, err := tags.Delete(ctx, "ghost"); errorCode(err) != apperr.NotFound ||
		!strings.Contains(err.Error(), "ghost") {
		t.Errorf("Delete(missing) error = %v, want not_found naming input", err)
	}
}

func TestTagTransactionBeginsImmediatelyAndRollsBack(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gsd.db")
	storage, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	tags := NewTags(storage)

	competing, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(competing) error = %v", err)
	}
	t.Cleanup(func() { _ = competing.Close() })
	if _, err := competing.database.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		t.Fatalf("disable competing busy timeout: %v", err)
	}

	rollback := errors.New("roll back tag transaction")
	err = tags.WithinTransaction(ctx, func(transaction tag.Transaction) error {
		if _, err := competing.database.ExecContext(ctx, "INSERT INTO tags (title) VALUES ('competing')"); err == nil {
			t.Error("competing write error = nil, want immediate transaction to reserve writer")
		}
		if _, err := transaction.Add(ctx, "temporary", "2026-01-01T00:00:00.000Z"); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTransaction() error = %v, want rollback marker", err)
	}
	if _, err := tags.Find(ctx, "temporary"); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(rolled-back tag) error = %v, want not_found", err)
	}
	if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM tags"); count != 0 {
		t.Errorf("tag count after rollback = %d, want 0", count)
	}
}
