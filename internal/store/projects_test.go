package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestProjectStoreRoundTripsEditsAndOrdersLists(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)

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

func TestProjectAddAppendsWithinArea(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	areas := NewAreas(storage)

	firstArea := addStoredArea(t, areas, area.AddFields{Title: "first area"})
	secondArea := addStoredArea(t, areas, area.AddFields{Title: "second area"})

	fixtures := []struct {
		title    string
		areaID   *int64
		position int64
	}{
		{title: "standalone first", position: 0},
		{title: "first area first", areaID: &firstArea.ID, position: 0},
		{title: "standalone second", position: 1},
		{title: "second area first", areaID: &secondArea.ID, position: 0},
		{title: "first area second", areaID: &firstArea.ID, position: 1},
	}
	for _, fixture := range fixtures {
		created, err := projects.Add(
			ctx,
			project.AddFields{AreaID: fixture.areaID, Title: fixture.title},
			"2026-01-02T00:00:00.000Z",
		)
		if err != nil {
			t.Fatalf("Add(%s) error = %v", fixture.title, err)
		}
		if !reflect.DeepEqual(created.AreaID, fixture.areaID) || created.Position != fixture.position {
			t.Errorf(
				"Add(%s) = area %v, position %d; want area %v, position %d",
				fixture.title,
				created.AreaID,
				created.Position,
				fixture.areaID,
				fixture.position,
			)
		}
	}
}

func TestProjectListReturnsEmptyForExistingFilteredArea(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	areas := NewAreas(storage)

	requestedArea := addStoredArea(t, areas, area.AddFields{Title: "requested"})
	otherArea := addStoredArea(t, areas, area.AddFields{Title: "other"})
	resolved := addStoredProject(t, projects, project.AddFields{AreaID: &requestedArea.ID, Title: "resolved"})
	if _, err := projects.Resolve(ctx, resolved.ID, project.ExitDone, "2026-01-03T00:00:00.000Z"); err != nil {
		t.Fatalf("Resolve(project) error = %v", err)
	}
	addStoredProject(t, projects, project.AddFields{AreaID: &otherArea.ID, Title: "other project"})

	open, err := projects.List(ctx, project.ListOptions{
		Status: project.ListStatusOpen,
		AreaID: &requestedArea.ID,
	})
	if err != nil {
		t.Fatalf("List(existing filtered-empty area) error = %v", err)
	}
	if len(open) != 0 {
		t.Errorf("List(existing filtered-empty area) = %#v, want empty", open)
	}
}

func TestProjectEditReparentsAtomicallyAndPreservesNoOpMembership(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	areas := NewAreas(storage)

	sourceArea := addStoredArea(t, areas, area.AddFields{Title: "source"})
	destinationArea := addStoredArea(t, areas, area.AddFields{Title: "destination"})
	addStoredProject(t, projects, project.AddFields{AreaID: &destinationArea.ID, Title: "destination sibling"})
	addStoredProject(t, projects, project.AddFields{Title: "standalone sibling"})
	created := addStoredProject(t, projects, project.AddFields{AreaID: &sourceArea.ID, Title: "moving"})
	resolved, err := projects.Resolve(ctx, created.ID, project.ExitDone, "2026-01-03T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Resolve(moving project) error = %v", err)
	}

	unchanged, err := projects.Edit(
		ctx,
		resolved.ID,
		project.EditFields{Area: project.AreaChange{Set: &sourceArea.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(same area) error = %v", err)
	}
	if !reflect.DeepEqual(unchanged, resolved) {
		t.Errorf("Edit(same area) = %#v, want unchanged %#v", unchanged, resolved)
	}

	moved, err := projects.Edit(
		ctx,
		resolved.ID,
		project.EditFields{Area: project.AreaChange{Set: &destinationArea.ID}},
		"2026-01-05T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(move) error = %v", err)
	}
	if moved.AreaID == nil || *moved.AreaID != destinationArea.ID ||
		moved.Position != 1 || moved.Status != "done" {
		t.Errorf("Edit(move) = %#v, want resolved project appended to destination area", moved)
	}

	cleared, err := projects.Edit(
		ctx,
		moved.ID,
		project.EditFields{Area: project.AreaChange{Clear: true}},
		"2026-01-07T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(clear area) error = %v", err)
	}
	if cleared.AreaID != nil || cleared.Position != 1 {
		t.Errorf("Edit(clear area) = %#v, want standalone append at position 1", cleared)
	}

	redundantClear, err := projects.Edit(
		ctx,
		moved.ID,
		project.EditFields{Area: project.AreaChange{Clear: true}},
		"2026-01-08T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(redundant clear) error = %v", err)
	}
	if !reflect.DeepEqual(redundantClear, cleared) {
		t.Errorf("Edit(redundant clear) = %#v, want no-op %#v", redundantClear, cleared)
	}
}

func TestProjectArchivedAreaGuardsLifecycleButAllowsDelete(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)

	container := addStoredArea(t, areas, area.AddFields{Title: "retired"})
	completeCandidate := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "complete"})
	cancelCandidate := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "cancel"})
	reopenCandidate := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "reopen"})
	deleteCandidate := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "delete"})
	if _, err := projects.Resolve(
		ctx,
		reopenCandidate.ID,
		project.ExitDone,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(reopen fixture) error = %v", err)
	}
	if _, err := areas.Archive(ctx, container.ID, "2026-01-03T00:00:00.000Z"); err != nil {
		t.Fatalf("Archive(container) error = %v", err)
	}

	for _, apply := range []func() error{
		func() error {
			_, err := projects.Resolve(ctx, completeCandidate.ID, project.ExitDone, "2026-01-04T00:00:00.000Z")
			return err
		},
		func() error {
			_, err := projects.Resolve(ctx, cancelCandidate.ID, project.ExitCancelled, "2026-01-04T00:00:00.000Z")
			return err
		},
		func() error {
			_, err := projects.Reopen(ctx, reopenCandidate.ID, "2026-01-04T00:00:00.000Z")
			return err
		},
	} {
		err := apply()
		assertArchivedAreaConflict(t, err, container.ID)
	}

	deleted, err := projects.Delete(ctx, deleteCandidate.ID)
	if err != nil || deleted.ID != deleteCandidate.ID {
		t.Errorf("Delete(project under archived area) = %#v, %v; want allowed", deleted, err)
	}
}

func assertArchivedAreaConflict(t *testing.T, err error, wantIDs ...int64) {
	t.Helper()

	if errorCode(err) != apperr.Conflict {
		t.Fatalf("error = %v, want conflict", err)
	}
	var archived *area.ArchivedAreasError
	if !errors.As(err, &archived) {
		t.Fatalf("error = %v, want archived-area metadata", err)
	}
	if !reflect.DeepEqual(archived.IDs, wantIDs) {
		t.Errorf("archived area IDs = %v, want %v", archived.IDs, wantIDs)
	}
	for _, id := range wantIDs {
		if !strings.Contains(err.Error(), fmt.Sprint(id)) {
			t.Errorf("error = %v, want area ID %d", err, id)
		}
	}
}

func TestProjectLifecycleTransactionSharesTimestampAndOrdersCancelledTasks(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	created, err := projects.Add(
		ctx,
		project.AddFields{Title: "release"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	first := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "first"})
	done := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "already done"})
	last := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "last"})
	done, err = tasks.Done(ctx, done.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Done(existing task) error = %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, `
UPDATE tasks
SET position = CASE id WHEN ? THEN 2 WHEN ? THEN 1 WHEN ? THEN 0 END
WHERE project_id = ?
`, first.ID, done.ID, last.ID, created.ID); err != nil {
		t.Fatalf("arrange task positions: %v", err)
	}
	untouchedProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "untouched"},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(untouched project) error = %v", err)
	}
	untouchedProjectTask := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &untouchedProject.ID, Title: "other project"},
	)
	untouchedInboxTask := addStoredTask(t, tasks, task.AddFields{Title: "inbox"})

	resolvedAt := "2026-01-03T04:05:06.789Z"
	var resolved project.Project
	var cancelled []task.Task
	err = projects.WithinTransaction(ctx, func(transaction project.Transaction) error {
		resolved, err = transaction.Resolve(ctx, created.ID, project.ExitDone, resolvedAt)
		if err != nil {
			return err
		}
		cancelled, err = transaction.CancelOpenTasks(ctx, created.ID, resolvedAt)
		return err
	})
	if err != nil {
		t.Fatalf("WithinTransaction(resolve) error = %v", err)
	}
	if resolved.Status != "done" || resolved.DoneAt == nil || *resolved.DoneAt != resolvedAt ||
		resolved.UpdatedAt != resolvedAt {
		t.Errorf("resolved project = %#v, want done with shared timestamp", resolved)
	}
	if len(cancelled) != 2 || cancelled[0].ID != last.ID || cancelled[1].ID != first.ID {
		t.Fatalf("cancelled tasks = %#v, want open tasks ordered by position then ID", cancelled)
	}
	for _, cancelledTask := range cancelled {
		if cancelledTask.Status != "cancelled" || cancelledTask.CancelledAt == nil ||
			*cancelledTask.CancelledAt != resolvedAt || cancelledTask.UpdatedAt != resolvedAt {
			t.Errorf("cancelled task = %#v, want shared cascade timestamp", cancelledTask)
		}
	}
	persistedDone, err := tasks.Find(ctx, done.ID)
	if err != nil {
		t.Fatalf("Find(existing done task) error = %v", err)
	}
	if !reflect.DeepEqual(persistedDone, done) {
		t.Errorf("existing done task after cascade = %#v, want untouched %#v", persistedDone, done)
	}
	for _, untouched := range []task.Task{untouchedProjectTask, untouchedInboxTask} {
		persisted, err := tasks.Find(ctx, untouched.ID)
		if err != nil {
			t.Fatalf("Find(untouched task %d) error = %v", untouched.ID, err)
		}
		if !reflect.DeepEqual(persisted, untouched) {
			t.Errorf("untouched task after cascade = %#v, want %#v", persisted, untouched)
		}
	}

	reopenedAt := "2026-01-04T00:00:00.000Z"
	reopened, err := projects.Reopen(ctx, created.ID, reopenedAt)
	if err != nil {
		t.Fatalf("Reopen(project) error = %v", err)
	}
	if reopened.Status != "open" || reopened.DoneAt != nil || reopened.CancelledAt != nil ||
		reopened.UpdatedAt != reopenedAt {
		t.Errorf("Reopen(project) = %#v, want only project exit cleared", reopened)
	}
	persistedCancelled, err := tasks.Find(ctx, first.ID)
	if err != nil {
		t.Fatalf("Find(cascade-cancelled task) error = %v", err)
	}
	if persistedCancelled.Status != "cancelled" {
		t.Errorf("task status after project reopen = %q, want cancelled", persistedCancelled.Status)
	}
	if _, err := projects.Reopen(ctx, created.ID, "2026-01-05T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Reopen(open project) error = %v, want conflict", err)
	}
	cancelledAt := "2026-01-06T00:00:00.000Z"
	cancelledProject, err := projects.Resolve(ctx, created.ID, project.ExitCancelled, cancelledAt)
	if err != nil {
		t.Fatalf("Resolve(cancelled project) error = %v", err)
	}
	if cancelledProject.Status != "cancelled" || cancelledProject.CancelledAt == nil ||
		*cancelledProject.CancelledAt != cancelledAt || cancelledProject.DoneAt != nil {
		t.Errorf("Resolve(cancelled project) = %#v, want selected cancelled exit", cancelledProject)
	}
	if _, err := projects.Resolve(
		ctx,
		created.ID,
		project.ExitDone,
		"2026-01-07T00:00:00.000Z",
	); errorCode(err) != apperr.Conflict {
		t.Errorf("Resolve(already resolved project) error = %v, want conflict", err)
	}
}

func TestProjectResolutionRollsBackWhenCascadeFails(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	created, err := projects.Add(ctx, project.AddFields{Title: "atomic"}, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	contained := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "contained"})
	if _, err := storage.database.ExecContext(ctx, `
CREATE TRIGGER fail_project_cascade
BEFORE UPDATE OF cancelled_at ON tasks
BEGIN
    SELECT RAISE(ABORT, 'forced cascade failure');
END
`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	err = projects.WithinTransaction(ctx, func(transaction project.Transaction) error {
		if _, err := transaction.Resolve(
			ctx,
			created.ID,
			project.ExitCancelled,
			"2026-01-02T00:00:00.000Z",
		); err != nil {
			return err
		}
		_, err := transaction.CancelOpenTasks(ctx, created.ID, "2026-01-02T00:00:00.000Z")
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "forced cascade failure") {
		t.Fatalf("WithinTransaction(failing cascade) error = %v, want trigger failure", err)
	}
	persistedProject, err := projects.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(project after rollback) error = %v", err)
	}
	persistedTask, err := tasks.Find(ctx, contained.ID)
	if err != nil {
		t.Fatalf("Find(task after rollback) error = %v", err)
	}
	if persistedProject.Status != "open" || persistedTask.Status != "open" {
		t.Errorf(
			"states after rollback = project %q, task %q; want both open",
			persistedProject.Status,
			persistedTask.Status,
		)
	}
}

func TestProjectTransactionRollsBackAfterPanic(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)

	created, err := projects.Add(
		ctx,
		project.AddFields{Title: "panic rollback"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}

	const panicValue = "forced transaction panic"
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		_ = projects.WithinTransaction(ctx, func(transaction project.Transaction) error {
			if _, err := transaction.Resolve(
				ctx,
				created.ID,
				project.ExitDone,
				"2026-01-02T00:00:00.000Z",
			); err != nil {
				return err
			}
			panic(panicValue)
		})
		t.Error("WithinTransaction(panic) returned, want panic")
	}()
	if recovered != panicValue {
		t.Fatalf("WithinTransaction(panic) recovered = %#v, want %q", recovered, panicValue)
	}

	persisted, err := projects.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(project after panic) error = %v", err)
	}
	if !reflect.DeepEqual(persisted, created) {
		t.Errorf("project after panic = %#v, want rolled-back %#v", persisted, created)
	}
	if _, err := projects.Add(
		ctx,
		project.AddFields{Title: "connection remains usable"},
		"2026-01-03T00:00:00.000Z",
	); err != nil {
		t.Errorf("Add(after transaction panic) error = %v", err)
	}
}

func TestProjectDeleteHonorsRestrictAndRecursiveTransaction(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	created, err := projects.Add(ctx, project.AddFields{Title: "doomed"}, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	first := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "first"})
	second := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "second"})
	third := addStoredTask(t, tasks, task.AddFields{ProjectID: &created.ID, Title: "third"})
	if _, err := tasks.Done(ctx, second.ID, "2026-01-02T00:00:00.000Z"); err != nil {
		t.Fatalf("Done(second) error = %v", err)
	}
	if _, err := tasks.Cancel(ctx, third.ID, "2026-01-03T00:00:00.000Z"); err != nil {
		t.Fatalf("Cancel(third) error = %v", err)
	}
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE tasks SET position = CASE id WHEN ? THEN 1 WHEN ? THEN 0 WHEN ? THEN 1 END",
		first.ID,
		second.ID,
		third.ID,
	); err != nil {
		t.Fatalf("arrange positions: %v", err)
	}
	untouchedProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "untouched"},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(untouched project) error = %v", err)
	}
	untouchedProjectTask := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &untouchedProject.ID, Title: "other project"},
	)
	untouchedInboxTask := addStoredTask(t, tasks, task.AddFields{Title: "inbox"})

	if _, err := projects.Delete(ctx, created.ID); errorCode(err) != apperr.Conflict ||
		!strings.Contains(err.Error(), "contains tasks") {
		t.Fatalf("Delete(containing project) error = %v, want containment conflict", err)
	}

	aborted, err := projects.Add(
		ctx,
		project.AddFields{Title: "non-FK failure"},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(non-FK failure project) error = %v", err)
	}
	nonFKTrigger := fmt.Sprintf(`
CREATE TRIGGER abort_project_delete
BEFORE DELETE ON projects
WHEN OLD.id = %d
BEGIN
    SELECT RAISE(ABORT, 'forced non-FK delete failure');
END
`, aborted.ID)
	if _, err := storage.database.ExecContext(ctx, nonFKTrigger); err != nil {
		t.Fatalf("create non-FK failure trigger: %v", err)
	}
	if _, err := projects.Delete(ctx, aborted.ID); err == nil ||
		!strings.Contains(err.Error(), "forced non-FK delete failure") {
		t.Fatalf("Delete(aborted non-FK) error = %v, want original trigger failure", err)
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("Delete(aborted non-FK) error = %v, want uncoded internal failure", err)
	}
	if _, err := storage.database.ExecContext(ctx, "DROP TRIGGER abort_project_delete"); err != nil {
		t.Fatalf("drop non-FK failure trigger: %v", err)
	}

	var deletedProject project.Project
	var deletedTasks []task.Task
	err = projects.WithinTransaction(ctx, func(transaction project.Transaction) error {
		deletedTasks, err = transaction.DeleteTasks(ctx, created.ID)
		if err != nil {
			return err
		}
		deletedProject, err = transaction.Delete(ctx, created.ID)
		return err
	})
	if err != nil {
		t.Fatalf("WithinTransaction(recursive delete) error = %v", err)
	}
	if !reflect.DeepEqual(deletedProject, created) {
		t.Errorf("deleted project = %#v, want snapshot %#v", deletedProject, created)
	}
	wantIDs := []int64{second.ID, first.ID, third.ID}
	gotIDs := make([]int64, len(deletedTasks))
	for index := range deletedTasks {
		gotIDs[index] = deletedTasks[index].ID
	}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("deleted task IDs = %v, want all states ordered by position then ID %v", gotIDs, wantIDs)
	}
	if _, err := projects.Find(ctx, created.ID); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(deleted project) error = %v, want not_found", err)
	}
	for _, deletedTask := range deletedTasks {
		if _, err := tasks.Find(ctx, deletedTask.ID); errorCode(err) != apperr.NotFound {
			t.Errorf("Find(deleted task %d) error = %v, want not_found", deletedTask.ID, err)
		}
	}
	for _, untouched := range []task.Task{untouchedProjectTask, untouchedInboxTask} {
		persisted, err := tasks.Find(ctx, untouched.ID)
		if err != nil {
			t.Fatalf("Find(untouched task %d) error = %v", untouched.ID, err)
		}
		if !reflect.DeepEqual(persisted, untouched) {
			t.Errorf("untouched task after recursive deletion = %#v, want %#v", persisted, untouched)
		}
	}

	rollbackProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "rollback"},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(rollback project) error = %v", err)
	}
	rollbackTask := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &rollbackProject.ID, Title: "must survive"},
	)
	trigger := fmt.Sprintf(`
CREATE TRIGGER fail_project_delete
BEFORE DELETE ON projects
WHEN OLD.id = %d
BEGIN
    SELECT RAISE(ABORT, 'forced project delete failure');
END
`, rollbackProject.ID)
	if _, err := storage.database.ExecContext(ctx, trigger); err != nil {
		t.Fatalf("create project delete trigger: %v", err)
	}
	err = projects.WithinTransaction(ctx, func(transaction project.Transaction) error {
		if _, err := transaction.DeleteTasks(ctx, rollbackProject.ID); err != nil {
			return err
		}
		_, err := transaction.Delete(ctx, rollbackProject.ID)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "forced project delete failure") {
		t.Fatalf("WithinTransaction(failing recursive delete) error = %v, want trigger failure", err)
	}
	if _, err := projects.Find(ctx, rollbackProject.ID); err != nil {
		t.Errorf("Find(project after recursive rollback) error = %v", err)
	}
	if _, err := tasks.Find(ctx, rollbackTask.ID); err != nil {
		t.Errorf("Find(task after recursive rollback) error = %v", err)
	}
}

func TestProjectBoardAndStageFindersUseStoredIdentityAndPosition(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	pipeline := addStoredBoard(
		t,
		boards,
		board.AddFields{Title: "Delivery"},
		"2026-01-01T00:00:00.000Z",
	)
	earlier := addStoredStage(t, boards, pipeline.ID, "Research", "2026-01-01T00:00:00.000Z")
	later := addStoredStage(t, boards, pipeline.ID, "Review", "2026-01-01T00:00:00.000Z")
	if _, err := boards.ReorderStage(
		ctx,
		pipeline.ID,
		later.ID,
		domain.Placement{Anchor: domain.PlacementFirst},
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("ReorderStage(later first) error = %v", err)
	}
	stageless := addStoredBoard(
		t,
		boards,
		board.AddFields{Title: "Empty"},
		"2026-01-01T00:00:00.000Z",
	)

	foundBoard, err := projects.FindBoard(ctx, "dElIvErY")
	if err != nil {
		t.Fatalf("FindBoard(case variant) error = %v", err)
	}
	if foundBoard.ID != pipeline.ID || foundBoard.Title != pipeline.Title {
		t.Errorf("FindBoard(case variant) = %#v, want stored identity %#v", foundBoard, pipeline)
	}

	first, err := projects.FindFirstStage(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("FindFirstStage(reordered board) error = %v", err)
	}
	if first == nil || first.ID != later.ID || first.BoardTitle != pipeline.Title ||
		first.Title != later.Title || first.Position != 0 {
		t.Errorf("FindFirstStage(reordered board) = %#v, want stored stage %q at position 0", first, later.Title)
	}
	if first, err := projects.FindFirstStage(ctx, stageless.ID); err != nil || first != nil {
		t.Errorf("FindFirstStage(stageless board) = %#v, %v; want nil, nil", first, err)
	}

	foundStage, err := projects.FindStage(ctx, pipeline.ID, "rEsEaRcH")
	if err != nil {
		t.Fatalf("FindStage(case variant) error = %v", err)
	}
	if foundStage.ID != earlier.ID || foundStage.BoardID != pipeline.ID ||
		foundStage.BoardTitle != pipeline.Title || foundStage.Title != earlier.Title ||
		foundStage.Position != 1 {
		t.Errorf("FindStage(case variant) = %#v, want stored stage %q at position 1", foundStage, earlier.Title)
	}
	foundByID, err := projects.FindStageByID(ctx, later.ID)
	if err != nil {
		t.Fatalf("FindStageByID() error = %v", err)
	}
	if foundByID.ID != later.ID || foundByID.BoardID != pipeline.ID ||
		foundByID.BoardTitle != pipeline.Title || foundByID.Title != later.Title ||
		foundByID.Position != 0 {
		t.Errorf("FindStageByID() = %#v, want stored reordered stage", foundByID)
	}

	if _, err := projects.FindBoard(ctx, "missing"); errorCode(err) != apperr.NotFound {
		t.Errorf("FindBoard(missing) error = %v, want not_found", err)
	}
	if _, err := projects.FindStage(ctx, pipeline.ID, "missing"); errorCode(err) != apperr.NotFound {
		t.Errorf("FindStage(missing) error = %v, want not_found", err)
	}
	if _, err := projects.FindStageByID(ctx, 999); errorCode(err) != apperr.NotFound {
		t.Errorf("FindStageByID(missing) error = %v, want not_found", err)
	}
}

func TestProjectStageMembershipAppendsClearsAndSwitches(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	firstStage := addStoredStage(t, boards, pipeline.ID, "first", "2026-01-01T00:00:00.000Z")
	secondStage := addStoredStage(t, boards, pipeline.ID, "second", "2026-01-01T00:00:00.000Z")

	first := addStoredProject(t, projects, project.AddFields{StageID: &firstStage.ID, Title: "first"})
	second := addStoredProject(t, projects, project.AddFields{StageID: &firstStage.ID, Title: "second"})
	anchor := addStoredProject(t, projects, project.AddFields{StageID: &secondStage.ID, Title: "anchor"})
	if first.StageID == nil || *first.StageID != firstStage.ID || first.StagePosition == nil || *first.StagePosition != 0 ||
		second.StagePosition == nil || *second.StagePosition != 1 || anchor.StagePosition == nil || *anchor.StagePosition != 0 {
		t.Fatalf("stage additions = %#v/%#v/%#v, want independently appended positions 0/1 and 0", first, second, anchor)
	}

	cleared, err := projects.Edit(
		ctx,
		first.ID,
		project.EditFields{Stage: project.StageChange{Clear: true}},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(clear stage) error = %v", err)
	}
	if cleared.StageID != nil || cleared.StagePosition != nil {
		t.Errorf("cleared project = %#v, want both stage fields null", cleared)
	}

	switched, err := projects.Edit(
		ctx,
		second.ID,
		project.EditFields{Stage: project.StageChange{Set: &secondStage.ID}},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(switch stage) error = %v", err)
	}
	if switched.StageID == nil || *switched.StageID != secondStage.ID ||
		switched.StagePosition == nil || *switched.StagePosition != 1 {
		t.Errorf("switched project = %#v, want append after target anchor", switched)
	}
	assertProjectStageOrder(t, storage, secondStage.ID, []int64{anchor.ID, second.ID})

	restated, err := projects.Edit(
		ctx,
		second.ID,
		project.EditFields{Stage: project.StageChange{Set: &secondStage.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate stage) error = %v", err)
	}
	if !reflect.DeepEqual(restated, switched) {
		t.Errorf("restated stage = %#v, want unchanged %#v", restated, switched)
	}
}

func TestProjectStageMovesAppendPlaceAndReorderWithMovedTimestampOnly(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	source := addStoredStage(t, boards, pipeline.ID, "source", "2026-01-01T00:00:00.000Z")
	target := addStoredStage(t, boards, pipeline.ID, "target", "2026-01-01T00:00:00.000Z")

	movingFirst := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "moving first"})
	movingSecond := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "moving second"})
	targetFirst := addStoredProject(t, projects, project.AddFields{StageID: &target.ID, Title: "target first"})
	targetSecond := addStoredProject(t, projects, project.AddFields{StageID: &target.ID, Title: "target second"})

	areaAxis := []int64{movingFirst.ID, movingSecond.ID, targetFirst.ID, targetSecond.ID}
	assertStoredOrder(t, storage, "projects", "area_id IS NULL", nil, areaAxis)

	appended, err := projects.MoveStage(
		ctx,
		movingFirst.ID,
		target.ID,
		domain.Placement{},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("MoveStage(cross-stage append) error = %v", err)
	}
	if appended.StageID == nil || *appended.StageID != target.ID ||
		appended.StagePosition == nil || *appended.StagePosition != 2 {
		t.Errorf("appended project = %#v, want target position 2", appended)
	}
	assertProjectStageOrder(t, storage, target.ID, []int64{targetFirst.ID, targetSecond.ID, movingFirst.ID})

	placed, err := projects.MoveStage(
		ctx,
		movingSecond.ID,
		target.ID,
		domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: targetSecond.ID},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("MoveStage(cross-stage placement) error = %v", err)
	}
	if placed.StageID == nil || *placed.StageID != target.ID ||
		placed.StagePosition == nil || *placed.StagePosition != 1 {
		t.Errorf("placed project = %#v, want target position 1", placed)
	}
	assertProjectStageOrder(t, storage, target.ID, []int64{
		targetFirst.ID, movingSecond.ID, targetSecond.ID, movingFirst.ID,
	})

	ids := []int64{targetFirst.ID, movingSecond.ID, targetSecond.ID, movingFirst.ID}
	before := storedUpdatedAt(t, storage, "projects", ids)
	movedAt := "2026-01-04T00:00:00.000Z"
	reordered, err := projects.MoveStage(
		ctx,
		movingFirst.ID,
		target.ID,
		domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: movingSecond.ID},
		movedAt,
	)
	if err != nil {
		t.Fatalf("MoveStage(within-stage reorder) error = %v", err)
	}
	if reordered.StagePosition == nil || *reordered.StagePosition != 1 {
		t.Errorf("reordered project = %#v, want target position 1", reordered)
	}
	assertProjectStageOrder(t, storage, target.ID, []int64{
		targetFirst.ID, movingFirst.ID, movingSecond.ID, targetSecond.ID,
	})
	assertMovedOnlyTimestamp(t, storage, "projects", before, movingFirst.ID, movedAt)
	assertStoredOrder(t, storage, "projects", "area_id IS NULL", nil, areaAxis)
}

func TestProjectStageMovesHonorBoundaryPlacementAndRollBackFailures(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	source := addStoredStage(t, boards, pipeline.ID, "source", "2026-01-01T00:00:00.000Z")
	target := addStoredStage(t, boards, pipeline.ID, "target", "2026-01-01T00:00:00.000Z")

	rollbackMover := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "rollback mover"})
	foreignReference := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "foreign reference"})
	movingFirst := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "moving first"})
	movingLast := addStoredProject(t, projects, project.AddFields{StageID: &source.ID, Title: "moving last"})
	targetFirst := addStoredProject(t, projects, project.AddFields{StageID: &target.ID, Title: "target first"})
	targetSecond := addStoredProject(t, projects, project.AddFields{StageID: &target.ID, Title: "target second"})

	_, err := projects.MoveStage(
		ctx,
		rollbackMover.ID,
		target.ID,
		domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: foreignReference.ID},
		"2026-01-02T00:00:00.000Z",
	)
	if errorCode(err) != apperr.InvalidArgument {
		t.Fatalf("MoveStage(foreign cross-stage reference) error = %v, want invalid_argument", err)
	}
	persistedMover, err := projects.Find(ctx, rollbackMover.ID)
	if err != nil {
		t.Fatalf("Find(rollback mover) error = %v", err)
	}
	if !reflect.DeepEqual(persistedMover, rollbackMover) {
		t.Errorf("rollback mover after failed move = %#v, want unchanged %#v", persistedMover, rollbackMover)
	}
	assertProjectStageOrder(t, storage, source.ID, []int64{
		rollbackMover.ID, foreignReference.ID, movingFirst.ID, movingLast.ID,
	})
	assertProjectStageOrder(t, storage, target.ID, []int64{targetFirst.ID, targetSecond.ID})

	first, err := projects.MoveStage(
		ctx,
		movingFirst.ID,
		target.ID,
		domain.Placement{Anchor: domain.PlacementFirst},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("MoveStage(cross-stage first) error = %v", err)
	}
	if first.StageID == nil || *first.StageID != target.ID ||
		first.StagePosition == nil || *first.StagePosition != 0 {
		t.Errorf("MoveStage(cross-stage first) = %#v, want target position 0", first)
	}
	assertProjectStageOrder(t, storage, target.ID, []int64{movingFirst.ID, targetFirst.ID, targetSecond.ID})

	last, err := projects.MoveStage(
		ctx,
		movingLast.ID,
		target.ID,
		domain.Placement{Anchor: domain.PlacementLast},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("MoveStage(cross-stage last) error = %v", err)
	}
	if last.StageID == nil || *last.StageID != target.ID ||
		last.StagePosition == nil || *last.StagePosition != 3 {
		t.Errorf("MoveStage(cross-stage last) = %#v, want target position 3", last)
	}
	assertProjectStageOrder(t, storage, target.ID, []int64{
		movingFirst.ID, targetFirst.ID, targetSecond.ID, movingLast.ID,
	})
}

func assertProjectStageOrder(t *testing.T, storage *DB, stageID int64, want []int64) {
	t.Helper()
	rows, err := storage.database.QueryContext(
		context.Background(),
		"SELECT id, stage_position FROM projects WHERE stage_id = ? ORDER BY stage_position, id",
		stageID,
	)
	if err != nil {
		t.Fatalf("query project stage order: %v", err)
	}
	defer func() { _ = rows.Close() }()

	got := make([]int64, 0, len(want))
	for rows.Next() {
		var id, position int64
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatalf("scan project stage order: %v", err)
		}
		if position != int64(len(got)) {
			t.Errorf("project %d stage_position = %d, want %d", id, position, len(got))
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate project stage order: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("project stage order = %v, want %v", got, want)
	}
}

func TestProjectStoreRejectsInvalidCallsAndReportsMissingProjects(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)

	if _, err := projects.Find(ctx, 99); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Find(missing) error = %v, want not_found wrapping sql.ErrNoRows", err)
	}
	if _, err := projects.Resolve(
		ctx,
		99,
		project.ExitDone,
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Resolve(missing) error = %v, want not_found", err)
	}
	if _, err := projects.Reopen(ctx, 99, "2026-01-01T00:00:00.000Z"); errorCode(err) != apperr.NotFound {
		t.Errorf("Reopen(missing) error = %v, want not_found", err)
	}
	if _, err := projects.Delete(ctx, 99); errorCode(err) != apperr.NotFound {
		t.Errorf("Delete(missing) error = %v, want not_found", err)
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
	areaID := int64(1)
	if _, err := projects.Edit(
		ctx,
		1,
		project.EditFields{Area: project.AreaChange{Set: &areaID, Clear: true}},
		"2026-01-01T00:00:00.000Z",
	); err == nil {
		t.Error("Edit(set and clear area) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("Edit(set and clear area) error = %v, want uncoded caller-contract error", err)
	}
	if _, err := projects.List(ctx, project.ListOptions{Status: project.ListStatus("invalid")}); err == nil {
		t.Error("List(invalid status) error = nil, want caller-contract error")
	} else if _, coded := apperr.CodeOf(err); coded {
		t.Errorf("List(invalid status) error = %v, want uncoded caller-contract error", err)
	}
}
