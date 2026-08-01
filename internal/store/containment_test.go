package store

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestTaskAddScopesPositionsByContainerAndFiltersByProject(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	firstProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "first project"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(first project) error = %v", err)
	}
	secondProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "second project"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(second project) error = %v", err)
	}

	looseFirst := addStoredTask(t, tasks, task.AddFields{Title: "loose first"})
	projectFirst := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &firstProject.ID, Title: "project first"},
	)
	looseSecond := addStoredTask(t, tasks, task.AddFields{Title: "loose second"})
	otherProjectFirst := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &secondProject.ID, Title: "other project first"},
	)
	projectSecond := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &firstProject.ID, Title: "project second"},
	)

	positions := []int64{
		looseFirst.Position,
		projectFirst.Position,
		looseSecond.Position,
		otherProjectFirst.Position,
		projectSecond.Position,
	}
	wantPositions := []int64{0, 0, 1, 0, 1}
	if !reflect.DeepEqual(positions, wantPositions) {
		t.Errorf("interleaved positions = %v, want independently scoped %v", positions, wantPositions)
	}
	if projectFirst.ProjectID == nil || *projectFirst.ProjectID != firstProject.ID {
		t.Errorf("project task project_id = %#v, want %d", projectFirst.ProjectID, firstProject.ID)
	}

	listed, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusAll,
		ProjectID: &firstProject.ID,
	})
	if err != nil {
		t.Fatalf("List(first project) error = %v", err)
	}
	if len(listed) != 2 || listed[0].ID != projectFirst.ID || listed[1].ID != projectSecond.ID {
		t.Errorf("List(first project) = %#v, want its two tasks in position order", listed)
	}

	missingProjectID := int64(99)
	if _, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusAll,
		ProjectID: &missingProjectID,
	}); errorCode(err) != apperr.NotFound {
		t.Errorf("List(missing project) error = %v, want not_found", err)
	}
}

func TestProjectTaskListDistinguishesMissingFromExistingFilteredEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	container, err := projects.Add(
		ctx,
		project.AddFields{Title: "container"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	contained := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &container.ID, Title: "undated open task"},
	)

	listed, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusOpen,
		ProjectID: &container.ID,
	})
	if err != nil {
		t.Fatalf("List(open project tasks) error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != contained.ID {
		t.Errorf("List(open project tasks) = %#v, want contained task", listed)
	}
	for _, options := range []task.ListOptions{
		{Status: task.ListStatusDone, ProjectID: &container.ID},
		{Status: task.ListStatusOpen, Date: task.DateSelectorDue, ProjectID: &container.ID},
		{Status: task.ListStatusAll, Date: task.DateSelectorOverdue, ProjectID: &container.ID},
		{Status: task.ListStatusAll, Date: task.DateSelectorDeferred, ProjectID: &container.ID},
	} {
		listed, err := tasks.List(ctx, options)
		if err != nil {
			t.Errorf("List(existing filtered-empty project, %#v) error = %v", options, err)
			continue
		}
		if len(listed) != 0 {
			t.Errorf("List(existing filtered-empty project, %#v) = %#v, want []", options, listed)
		}
	}

	missingID := int64(99)
	if _, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusDone,
		ProjectID: &missingID,
	}); errorCode(err) != apperr.NotFound {
		t.Errorf("List(missing filtered-empty project) error = %v, want not_found", err)
	}
}

func TestTaskAddClassifiesMissingAndResolvedProjects(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	missingProjectID := int64(99)
	if _, err := tasks.Add(
		ctx,
		task.AddFields{ProjectID: &missingProjectID, Title: "missing"},
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Add(missing project) error = %v, want not_found", err)
	}

	resolved, err := projects.Add(
		ctx,
		project.AddFields{Title: "resolved"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET cancelled_at = ? WHERE id = ?",
		"2026-01-02T00:00:00.000Z",
		resolved.ID,
	); err != nil {
		t.Fatalf("resolve project fixture: %v", err)
	}
	_, err = tasks.Add(
		ctx,
		task.AddFields{ProjectID: &resolved.ID, Title: "blocked"},
		"2026-01-03T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), "reopen the project first") {
		t.Errorf("Add(resolved project) error = %v, want conflict with reopen guidance", err)
	}

	var taskCount int
	if err := storage.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Errorf("task count = %d, want no inserts from rejected additions", taskCount)
	}
}

func TestTaskEditReparentsByAppendingAndPreservesSameContainerNoOp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	firstProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "first"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(first project) error = %v", err)
	}
	secondProject, err := projects.Add(
		ctx,
		project.AddFields{Title: "second"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(second project) error = %v", err)
	}

	loose := addStoredTask(t, tasks, task.AddFields{Title: "loose"})
	moving := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "moving"})
	_ = addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "first sibling"})
	_ = addStoredTask(t, tasks, task.AddFields{ProjectID: &secondProject.ID, Title: "destination sibling"})

	moved, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &secondProject.ID}},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(move to second project) error = %v", err)
	}
	if moved.ProjectID == nil || *moved.ProjectID != secondProject.ID || moved.Position != 1 ||
		moved.UpdatedAt != "2026-01-02T00:00:00.000Z" {
		t.Errorf("moved task = %#v, want second project appended at position 1", moved)
	}

	restated, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &secondProject.ID}},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate second project) error = %v", err)
	}
	if !reflect.DeepEqual(restated, moved) {
		t.Errorf("same-container edit = %#v, want true no-op %#v", restated, moved)
	}

	title := "revised while restated"
	contentEdited, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{
			Project: task.ProjectChange{Set: &secondProject.ID},
			Title:   &title,
		},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content and same project) error = %v", err)
	}
	if contentEdited.Title != title || contentEdited.Position != moved.Position ||
		contentEdited.UpdatedAt != "2026-01-04T00:00:00.000Z" {
		t.Errorf("content edit with restated project = %#v, want content timestamp without movement", contentEdited)
	}

	cleared, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Clear: true}},
		"2026-01-05T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(clear project) error = %v", err)
	}
	if cleared.ProjectID != nil || cleared.Position != loose.Position+1 {
		t.Errorf("cleared task = %#v, want append after loose inbox task", cleared)
	}

	restatedInbox, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Clear: true}},
		"2026-01-06T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate inbox) error = %v", err)
	}
	if !reflect.DeepEqual(restatedInbox, cleared) {
		t.Errorf("same-inbox edit = %#v, want true no-op %#v", restatedInbox, cleared)
	}
}

func TestTaskEditEnforcesResolvedProjectMembershipGuards(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	source, err := projects.Add(
		ctx,
		project.AddFields{Title: "source"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(source project) error = %v", err)
	}
	target, err := projects.Add(
		ctx,
		project.AddFields{Title: "target"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(target project) error = %v", err)
	}
	contained := addStoredTask(t, tasks, task.AddFields{ProjectID: &source.ID, Title: "contained"})
	loose := addStoredTask(t, tasks, task.AddFields{Title: "loose"})

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET done_at = ? WHERE id = ?",
		"2026-01-02T00:00:00.000Z",
		source.ID,
	); err != nil {
		t.Fatalf("resolve source project: %v", err)
	}

	title := "content remains editable"
	contentEdited, err := tasks.Edit(
		ctx,
		contained.ID,
		task.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content in resolved project) error = %v", err)
	}
	if contentEdited.Title != title {
		t.Errorf("content edit title = %q, want %q", contentEdited.Title, title)
	}
	restated, err := tasks.Edit(
		ctx,
		contained.ID,
		task.EditFields{Project: task.ProjectChange{Set: &source.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate resolved project) error = %v", err)
	}
	if !reflect.DeepEqual(restated, contentEdited) {
		t.Errorf("restated resolved membership = %#v, want no-op %#v", restated, contentEdited)
	}

	for name, fields := range map[string]task.EditFields{
		"move out": {Project: task.ProjectChange{Clear: true}},
		"move between": {
			Project: task.ProjectChange{Set: &target.ID},
		},
	} {
		_, editErr := tasks.Edit(ctx, contained.ID, fields, "2026-01-05T00:00:00.000Z")
		if errorCode(editErr) != apperr.Conflict || !strings.Contains(editErr.Error(), "reopen the project first") {
			t.Errorf("Edit(%s resolved source) error = %v, want conflict with reopen guidance", name, editErr)
		}
	}

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET cancelled_at = ? WHERE id = ?",
		"2026-01-06T00:00:00.000Z",
		target.ID,
	); err != nil {
		t.Fatalf("resolve target project: %v", err)
	}
	blockedTitle := "must not persist"
	if _, err := tasks.Edit(
		ctx,
		loose.ID,
		task.EditFields{
			Project: task.ProjectChange{Set: &target.ID},
			Title:   &blockedTitle,
		},
		"2026-01-07T00:00:00.000Z",
	); errorCode(err) != apperr.Conflict || !strings.Contains(err.Error(), "reopen the project first") {
		t.Errorf("Edit(move into resolved target) error = %v, want conflict with reopen guidance", err)
	}
	unchanged, err := tasks.Find(ctx, loose.ID)
	if err != nil {
		t.Fatalf("Find(task after rejected move) error = %v", err)
	}
	if unchanged.Title != loose.Title || unchanged.ProjectID != nil || unchanged.UpdatedAt != loose.UpdatedAt {
		t.Errorf("task after rejected move = %#v, want unchanged %#v", unchanged, loose)
	}

	missingProjectID := int64(99)
	if _, err := tasks.Edit(
		ctx,
		loose.ID,
		task.EditFields{Project: task.ProjectChange{Set: &missingProjectID}},
		"2026-01-08T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(move into missing target) error = %v, want not_found", err)
	}
}

func TestTaskMoveReportsBothResolvedProjectsTogether(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	source, err := projects.Add(
		ctx,
		project.AddFields{Title: "source"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(source) error = %v", err)
	}
	destination, err := projects.Add(
		ctx,
		project.AddFields{Title: "destination"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(destination) error = %v", err)
	}
	moving := addStoredTask(t, tasks, task.AddFields{ProjectID: &source.ID, Title: "moving"})
	if _, err := projects.Resolve(
		ctx,
		source.ID,
		project.ExitDone,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(source) error = %v", err)
	}
	if _, err := projects.Resolve(
		ctx,
		destination.ID,
		project.ExitCancelled,
		"2026-01-03T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(destination) error = %v", err)
	}

	_, err = tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &destination.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict ||
		!strings.Contains(err.Error(), fmt.Sprint(source.ID)) ||
		!strings.Contains(err.Error(), fmt.Sprint(destination.ID)) ||
		!strings.Contains(err.Error(), "reopen both projects") {
		t.Errorf("Edit(between two resolved projects) error = %v, want both IDs and reopen-both guidance", err)
	}

	for _, id := range []int64{source.ID, destination.ID} {
		if _, err := projects.Reopen(ctx, id, "2026-01-05T00:00:00.000Z"); err != nil {
			t.Fatalf("Reopen(project %d) error = %v", id, err)
		}
	}
	moved, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &destination.ID}},
		"2026-01-06T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(after reopening both projects) error = %v", err)
	}
	if moved.ProjectID == nil || *moved.ProjectID != destination.ID {
		t.Errorf("Edit(after reopening both projects) = %#v, want destination project %d", moved, destination.ID)
	}
}

func addStoredTask(t *testing.T, tasks *Tasks, fields task.AddFields) task.Task {
	t.Helper()

	created, err := tasks.Add(context.Background(), fields, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(%q) error = %v", fields.Title, err)
	}

	return created
}
