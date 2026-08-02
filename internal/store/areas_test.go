package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
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
	if _, err := areas.Delete(ctx, 99); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Delete(missing) error = %v, want not_found wrapping sql.ErrNoRows", err)
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

func TestAreaArchiveLifecyclePreservesRowAndClassifiesInvalidTransitions(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	created := addStoredArea(t, areas, area.AddFields{Title: "Home", Note: "history"})

	archivedAt := "2026-01-02T00:00:00.000Z"
	archived, err := areas.Archive(ctx, created.ID, archivedAt)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.ArchivedAt == nil || *archived.ArchivedAt != archivedAt ||
		archived.UpdatedAt != archivedAt || archived.Title != created.Title ||
		archived.Note != created.Note || archived.Position != created.Position ||
		archived.CreatedAt != created.CreatedAt {
		t.Errorf("Archive() = %#v, want timestamped archive with stable content and position", archived)
	}
	if _, err := areas.Archive(ctx, created.ID, "2026-01-03T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Archive(already archived) error = %v, want conflict", err)
	}

	unarchivedAt := "2026-01-04T00:00:00.000Z"
	unarchived, err := areas.Unarchive(ctx, created.ID, unarchivedAt)
	if err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}
	if unarchived.ArchivedAt != nil || unarchived.UpdatedAt != unarchivedAt ||
		unarchived.Position != created.Position || unarchived.Title != created.Title ||
		unarchived.Note != created.Note || unarchived.CreatedAt != created.CreatedAt {
		t.Errorf("Unarchive() = %#v, want active row restored in place", unarchived)
	}
	if _, err := areas.Unarchive(ctx, created.ID, "2026-01-05T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Unarchive(active) error = %v, want conflict", err)
	}
	for _, apply := range []func() error{
		func() error {
			_, err := areas.Archive(ctx, 99, archivedAt)
			return err
		},
		func() error {
			_, err := areas.Unarchive(ctx, 99, unarchivedAt)
			return err
		},
	} {
		if err := apply(); errorCode(err) != apperr.NotFound {
			t.Errorf("missing lifecycle error = %v, want not_found", err)
		}
	}
}

func TestAreaDeleteHonorsRestrictAndRecursiveTransaction(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	doomed := addStoredArea(t, areas, area.AddFields{Title: "Doomed"})
	untouched := addStoredArea(t, areas, area.AddFields{Title: "Untouched"})
	firstProject := addStoredProject(t, projects, project.AddFields{AreaID: &doomed.ID, Title: "first project"})
	secondProject := addStoredProject(t, projects, project.AddFields{AreaID: &doomed.ID, Title: "second project"})
	firstProjectTask := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "first project task"})
	secondProjectTask := addStoredTask(t, tasks, task.AddFields{ProjectID: &secondProject.ID, Title: "second project task"})
	firstLoose := addStoredTask(t, tasks, task.AddFields{AreaID: &doomed.ID, Title: "first loose"})
	secondLoose := addStoredTask(t, tasks, task.AddFields{AreaID: &doomed.ID, Title: "second loose"})
	untouchedProject := addStoredProject(t, projects, project.AddFields{AreaID: &untouched.ID, Title: "untouched project"})
	untouchedTask := addStoredTask(t, tasks, task.AddFields{ProjectID: &untouchedProject.ID, Title: "untouched task"})
	if _, err := projects.Resolve(
		ctx,
		firstProject.ID,
		project.ExitDone,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(contained project) error = %v", err)
	}
	archivedDoomed, err := areas.Archive(ctx, doomed.ID, "2026-01-03T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Archive(doomed area) error = %v", err)
	}
	doomed = archivedDoomed

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET position = CASE id WHEN ? THEN 1 WHEN ? THEN 0 END WHERE area_id = ?",
		firstProject.ID,
		secondProject.ID,
		doomed.ID,
	); err != nil {
		t.Fatalf("arrange project deletion positions: %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, `
UPDATE tasks SET position = CASE id
    WHEN ? THEN 1 WHEN ? THEN 0 WHEN ? THEN 1 WHEN ? THEN 0 ELSE position END
WHERE project_id IN (?, ?) OR area_id = ?
`, firstProjectTask.ID, secondProjectTask.ID, firstLoose.ID, secondLoose.ID,
		firstProject.ID, secondProject.ID, doomed.ID); err != nil {
		t.Fatalf("arrange task deletion positions: %v", err)
	}

	if _, err := areas.Delete(ctx, doomed.ID); errorCode(err) != apperr.Conflict {
		t.Fatalf("Delete(nonempty) error = %v, want conflict", err)
	}

	var deletedArea area.Area
	var deletedProjects []project.Project
	var deletedProjectTasks []task.Task
	var deletedLooseTasks []task.Task
	err = areas.WithinTransaction(ctx, func(transaction area.Store) error {
		var operationErr error
		deletedProjectTasks, operationErr = transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeProject)
		if operationErr != nil {
			return operationErr
		}
		deletedProjects, operationErr = transaction.DeleteProjects(ctx, doomed.ID)
		if operationErr != nil {
			return operationErr
		}
		deletedLooseTasks, operationErr = transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeLoose)
		if operationErr != nil {
			return operationErr
		}
		deletedArea, operationErr = transaction.Delete(ctx, doomed.ID)
		return operationErr
	})
	if err != nil {
		t.Fatalf("WithinTransaction(recursive delete) error = %v", err)
	}
	if !reflect.DeepEqual(deletedArea, doomed) {
		t.Errorf("deleted area = %#v, want %#v", deletedArea, doomed)
	}
	if got := projectIDs(deletedProjects); !reflect.DeepEqual(got, []int64{secondProject.ID, firstProject.ID}) {
		t.Errorf("deleted project IDs = %v, want position/ID order", got)
	}
	if got := taskIDs(deletedProjectTasks); !reflect.DeepEqual(got, []int64{secondProjectTask.ID, firstProjectTask.ID}) {
		t.Errorf("deleted project task IDs = %v, want position/ID order", got)
	}
	if got := taskIDs(deletedLooseTasks); !reflect.DeepEqual(got, []int64{secondLoose.ID, firstLoose.ID}) {
		t.Errorf("deleted loose task IDs = %v, want position/ID order", got)
	}
	if _, err := areas.Find(ctx, doomed.ID); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(deleted area) error = %v, want not_found", err)
	}
	if _, err := projects.Find(ctx, untouchedProject.ID); err != nil {
		t.Errorf("Find(untouched project) error = %v", err)
	}
	if _, err := tasks.Find(ctx, untouchedTask.ID); err != nil {
		t.Errorf("Find(untouched task) error = %v", err)
	}

	emptyArchived := addStoredArea(t, areas, area.AddFields{Title: "Empty archived"})
	if _, err := areas.Archive(ctx, emptyArchived.ID, "2026-01-05T00:00:00.000Z"); err != nil {
		t.Fatalf("Archive(empty) error = %v", err)
	}
	if _, err := areas.Delete(ctx, emptyArchived.ID); err != nil {
		t.Errorf("Delete(empty archived) error = %v, want allowed", err)
	}
}

func TestAreaRecursiveDeleteRollsBackAllLevelsOnFailure(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	doomed := addStoredArea(t, areas, area.AddFields{Title: "Rollback"})
	containedProject := addStoredProject(t, projects, project.AddFields{AreaID: &doomed.ID, Title: "project"})
	projectTask := addStoredTask(t, tasks, task.AddFields{ProjectID: &containedProject.ID, Title: "project task"})
	looseTask := addStoredTask(t, tasks, task.AddFields{AreaID: &doomed.ID, Title: "loose task"})
	trigger := fmt.Sprintf(`
CREATE TRIGGER fail_area_delete
BEFORE DELETE ON areas
WHEN OLD.id = %d
BEGIN
    SELECT RAISE(ABORT, 'forced area delete failure');
END
`, doomed.ID)
	if _, err := storage.database.ExecContext(ctx, trigger); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	err := areas.WithinTransaction(ctx, func(transaction area.Store) error {
		if _, err := transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeProject); err != nil {
			return err
		}
		if _, err := transaction.DeleteProjects(ctx, doomed.ID); err != nil {
			return err
		}
		if _, err := transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeLoose); err != nil {
			return err
		}
		_, err := transaction.Delete(ctx, doomed.ID)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "forced area delete failure") {
		t.Fatalf("WithinTransaction(failing delete) error = %v, want trigger failure", err)
	}
	if _, err := areas.Find(ctx, doomed.ID); err != nil {
		t.Errorf("Find(area after rollback) error = %v", err)
	}
	if _, err := projects.Find(ctx, containedProject.ID); err != nil {
		t.Errorf("Find(project after rollback) error = %v", err)
	}
	for _, id := range []int64{projectTask.ID, looseTask.ID} {
		if _, err := tasks.Find(ctx, id); err != nil {
			t.Errorf("Find(task %d after rollback) error = %v", id, err)
		}
	}

	if _, err := areas.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScope("invalid")); err == nil {
		t.Error("DeleteTasks(invalid scope) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("DeleteTasks(invalid scope) error = %v, want uncoded caller-contract error", err)
	}
}

func projectIDs(values []project.Project) []int64 {
	ids := make([]int64, len(values))
	for index := range values {
		ids[index] = values[index].ID
	}
	return ids
}

func taskIDs(values []task.Task) []int64 {
	ids := make([]int64, len(values))
	for index := range values {
		ids[index] = values[index].ID
	}
	return ids
}

func addStoredArea(t *testing.T, areas *Areas, fields area.AddFields) area.Area {
	t.Helper()

	created, err := areas.Add(context.Background(), fields, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(%q) error = %v", fields.Title, err)
	}
	return created
}
