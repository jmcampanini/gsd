package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
)

func TestProjectStoreRoundTripsEditsAndOrdersLists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	t.Cleanup(func() { _ = storage.Close() })

	first, err := projects.Add(
		ctx,
		project.AddFields{Title: "first", Note: "first note"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if first.Status != "open" || first.Position != 0 ||
		first.CreatedAt != "2026-01-01T00:00:00.000Z" || first.UpdatedAt != first.CreatedAt {
		t.Errorf("first project = %#v, want open project at position 0 with supplied timestamps", first)
	}

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET done_at = ? WHERE id = ?",
		"2026-01-02T00:00:00.000Z",
		first.ID,
	); err != nil {
		t.Fatalf("resolve first project: %v", err)
	}
	second, err := projects.Add(
		ctx,
		project.AddFields{Title: "second"},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	if second.Position != 1 {
		t.Errorf("second position = %d, want append after resolved project at 1", second.Position)
	}

	title := "  revised  "
	note := "details"
	edited, err := projects.Edit(
		ctx,
		second.ID,
		project.EditFields{Title: &title, Note: &note},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(second) error = %v", err)
	}
	if edited.Title != title || edited.Note != note || edited.Position != second.Position ||
		edited.CreatedAt != second.CreatedAt || edited.UpdatedAt != "2026-01-04T00:00:00.000Z" {
		t.Errorf("Edit(second) = %#v, want changed content and preserved stable fields", edited)
	}
	found, err := projects.Find(ctx, second.ID)
	if err != nil {
		t.Fatalf("Find(second) error = %v", err)
	}
	if !reflect.DeepEqual(found, edited) {
		t.Errorf("Find(second) = %#v, want edited project %#v", found, edited)
	}

	open, err := projects.List(ctx, project.ListOptions{Status: project.ListStatusOpen})
	if err != nil {
		t.Fatalf("List(open) error = %v", err)
	}
	if len(open) != 1 || open[0].ID != second.ID {
		t.Errorf("List(open) = %#v, want only second project", open)
	}
	all, err := projects.List(ctx, project.ListOptions{Status: project.ListStatusAll})
	if err != nil {
		t.Fatalf("List(all) error = %v", err)
	}
	if len(all) != 2 || all[0].ID != first.ID || all[1].ID != second.ID {
		t.Errorf("List(all) = %#v, want position-ordered first and second projects", all)
	}
}

func TestProjectStoreRejectsInvalidCallsAndReportsMissingProjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	t.Cleanup(func() { _ = storage.Close() })

	if _, err := projects.Find(ctx, 99); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Find(missing) error = %v, want not_found wrapping sql.ErrNoRows", err)
	}
	title := "missing"
	if _, err := projects.Edit(
		ctx,
		99,
		project.EditFields{Title: &title},
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(missing) error = %v, want not_found", err)
	}
	if _, err := projects.Edit(
		ctx,
		1,
		project.EditFields{},
		"2026-01-01T00:00:00.000Z",
	); err == nil {
		t.Error("Edit(no fields) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("Edit(no fields) error = %v, want uncoded caller-contract error", err)
	}
	if _, err := projects.List(ctx, project.ListOptions{Status: project.ListStatus("invalid")}); err == nil {
		t.Error("List(invalid status) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("List(invalid status) error = %v, want uncoded caller-contract error", err)
	}
}
