package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
)

func TestAreaStoreCRUDPreservesRowsAndAppendsGlobally(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	areas := NewAreas(storage)

	first, err := areas.Add(
		ctx,
		area.AddFields{Title: "Home", Note: "household"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if first.ID <= 0 || first.Title != "Home" || first.Note != "household" || first.ArchivedAt != nil ||
		first.Position != 0 || first.CreatedAt != "2026-01-01T00:00:00.000Z" ||
		first.UpdatedAt != first.CreatedAt {
		t.Errorf("Add(first) = %#v, want complete active row at position 0", first)
	}

	archivedAt := "2026-01-02T00:00:00.000Z"
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE areas SET archived_at = ? WHERE id = ?",
		archivedAt,
		first.ID,
	); err != nil {
		t.Fatalf("archive fixture: %v", err)
	}
	second, err := areas.Add(
		ctx,
		area.AddFields{Title: "Health"},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	if second.Position != 1 {
		t.Errorf("second position = %d, want global append past archived row at 1", second.Position)
	}

	title := "  Wellbeing  "
	note := "line one\nline two\n"
	edited, err := areas.Edit(
		ctx,
		second.ID,
		area.EditFields{Title: &title, Note: &note},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(second) error = %v", err)
	}
	if edited.Title != title || edited.Note != note || edited.Position != second.Position ||
		edited.CreatedAt != second.CreatedAt || edited.UpdatedAt != "2026-01-04T00:00:00.000Z" {
		t.Errorf("Edit(second) = %#v, want changed content and stable position/creation", edited)
	}
	found, err := areas.Find(ctx, second.ID)
	if err != nil {
		t.Fatalf("Find(second) error = %v", err)
	}
	if !reflect.DeepEqual(found, edited) {
		t.Errorf("Find(second) = %#v, want edited row %#v", found, edited)
	}
}

func TestAreaStoreListSlicesOrderByPositionThenID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	areas := NewAreas(storage)

	first := addStoredArea(t, areas, area.AddFields{Title: "first"})
	archived := addStoredArea(t, areas, area.AddFields{Title: "archived"})
	last := addStoredArea(t, areas, area.AddFields{Title: "last"})
	if _, err := storage.database.ExecContext(ctx, `
UPDATE areas
SET archived_at = CASE WHEN id = ? THEN '2026-01-02T00:00:00.000Z' ELSE archived_at END,
    position = CASE id WHEN ? THEN 1 WHEN ? THEN 0 WHEN ? THEN 1 END
WHERE id IN (?, ?, ?)
`, archived.ID, first.ID, archived.ID, last.ID, first.ID, archived.ID, last.ID); err != nil {
		t.Fatalf("arrange list fixtures: %v", err)
	}

	tests := []struct {
		name    string
		slice   area.ListSlice
		wantIDs []int64
	}{
		{name: "active", slice: area.ListSliceActive, wantIDs: []int64{first.ID, last.ID}},
		{name: "archived", slice: area.ListSliceArchived, wantIDs: []int64{archived.ID}},
		{name: "all", slice: area.ListSliceAll, wantIDs: []int64{archived.ID, first.ID, last.ID}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			listed, listErr := areas.List(ctx, area.ListOptions{Slice: test.slice})
			if listErr != nil {
				t.Fatalf("List(%s) error = %v", test.slice, listErr)
			}
			gotIDs := make([]int64, len(listed))
			for index := range listed {
				gotIDs[index] = listed[index].ID
			}
			if !reflect.DeepEqual(gotIDs, test.wantIDs) {
				t.Errorf("List(%s) IDs = %v, want position/ID order %v", test.slice, gotIDs, test.wantIDs)
			}
			for _, listedArea := range listed {
				if listedArea.ID == archived.ID && (listedArea.ArchivedAt == nil || *listedArea.ArchivedAt != "2026-01-02T00:00:00.000Z") {
					t.Errorf("archived row = %#v, want archived timestamp", listedArea)
				}
			}
		})
	}
}

func TestAreaStoreReportsMissingRowsAndRejectsInvalidCalls(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	areas := NewAreas(storage)

	if _, err := areas.Find(ctx, 99); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Find(missing) error = %v, want not_found wrapping sql.ErrNoRows", err)
	}
	title := "missing"
	if _, err := areas.Edit(
		ctx,
		99,
		area.EditFields{Title: &title},
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Edit(missing) error = %v, want not_found wrapping sql.ErrNoRows", err)
	}
	if _, err := areas.Edit(
		ctx,
		1,
		area.EditFields{},
		"2026-01-01T00:00:00.000Z",
	); err == nil {
		t.Error("Edit(no fields) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("Edit(no fields) error = %v, want uncoded caller-contract error", err)
	}
	if _, err := areas.List(ctx, area.ListOptions{Slice: area.ListSlice("invalid")}); err == nil {
		t.Error("List(invalid slice) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("List(invalid slice) error = %v, want uncoded caller-contract error", err)
	}
	if _, err := areas.List(ctx, area.ListOptions{}); err == nil {
		t.Error("List(empty slice) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("List(empty slice) error = %v, want uncoded caller-contract error", err)
	}
}

func addStoredArea(t *testing.T, areas *Areas, fields area.AddFields) area.Area {
	t.Helper()

	created, err := areas.Add(context.Background(), fields, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(%q) error = %v", fields.Title, err)
	}
	return created
}
