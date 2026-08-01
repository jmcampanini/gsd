package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestMilestoneThreeSchemaColumnsConstraintsAndProjectIndex(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	projectColumns := schemaColumnNames(t, storage.database, "projects")
	wantProjectColumns := []string{
		"id", "title", "note", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at",
	}
	if !reflect.DeepEqual(projectColumns, wantProjectColumns) {
		t.Errorf("projects columns = %v, want %v", projectColumns, wantProjectColumns)
	}
	taskColumns := schemaColumnNames(t, storage.database, "tasks")
	wantTaskColumns := []string{
		"id", "project_id", "title", "note", "defer_until", "due_on", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at",
	}
	if !reflect.DeepEqual(taskColumns, wantTaskColumns) {
		t.Errorf("tasks columns = %v, want %v", taskColumns, wantTaskColumns)
	}

	var projectIndexCount int
	if err := storage.database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pragma_index_list('tasks')
WHERE name = 'idx_tasks_project'
`).Scan(&projectIndexCount); err != nil {
		t.Fatalf("inspect task project index: %v", err)
	}
	if projectIndexCount != 1 {
		t.Errorf("idx_tasks_project count = %d, want 1", projectIndexCount)
	}

	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (title, done_at, cancelled_at, position)
VALUES ('invalid state', '2026-01-01T00:00:00.000Z', '2026-01-02T00:00:00.000Z', 0)
`); err == nil {
		t.Error("insert project with both exits error = nil, want CHECK failure")
	}
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO tasks (project_id, title, position) VALUES (99, 'orphan', 0)
`); err == nil {
		t.Error("insert task with missing project error = nil, want FK failure")
	}

	result, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (title, position) VALUES ('parent', 0)
`)
	if err != nil {
		t.Fatalf("insert parent project: %v", err)
	}
	projectID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read parent project ID: %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO tasks (project_id, title, position) VALUES (?, 'child', 0)
`, projectID); err != nil {
		t.Fatalf("insert child task: %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", projectID); err == nil {
		t.Error("delete nonempty project error = nil, want RESTRICT failure")
	}
}

func TestMilestoneThreeViewsApplyContainmentAndExposeLogbookContract(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	parent, err := projects.Add(
		ctx,
		project.AddFields{Title: "parent"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	contained, err := tasks.Add(
		ctx,
		task.AddFields{ProjectID: &parent.ID, Title: "contained"},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(contained task) error = %v", err)
	}
	loose, err := tasks.Add(
		ctx,
		task.AddFields{Title: "loose"},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(loose task) error = %v", err)
	}

	inbox, err := tasks.Inbox(ctx)
	if err != nil {
		t.Fatalf("Inbox() error = %v", err)
	}
	if len(inbox) != 1 || inbox[0].ID != loose.ID {
		t.Errorf("Inbox() = %#v, want only loose task", inbox)
	}

	var projectTitle string
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT project_title FROM available WHERE id = ?",
		contained.ID,
	).Scan(&projectTitle); err != nil {
		t.Fatalf("query available project title: %v", err)
	}
	if projectTitle != parent.Title {
		t.Errorf("available project title = %q, want %q", projectTitle, parent.Title)
	}

	projectResolvedAt := "2026-01-04T00:00:00.000Z"
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET done_at = ? WHERE id = ?",
		projectResolvedAt,
		parent.ID,
	); err != nil {
		t.Fatalf("resolve project fixture: %v", err)
	}
	available, err := tasks.Available(ctx)
	if err != nil {
		t.Fatalf("Available() error = %v", err)
	}
	if len(available) != 1 || available[0].ID != loose.ID {
		t.Errorf("Available() = %#v, want loose task and no task in resolved project", available)
	}

	taskResolvedAt := "2026-01-05T00:00:00.000Z"
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE tasks SET cancelled_at = ? WHERE id = ?",
		taskResolvedAt,
		contained.ID,
	); err != nil {
		t.Fatalf("resolve task fixture: %v", err)
	}

	logbookColumns := schemaColumnNames(t, storage.database, "logbook")
	wantLogbookColumns := []string{"kind", "id", "title", "status", "resolved_at", "project_title"}
	if !reflect.DeepEqual(logbookColumns, wantLogbookColumns) {
		t.Errorf("logbook columns = %v, want %v", logbookColumns, wantLogbookColumns)
	}
	var logbookSQL string
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT sql FROM sqlite_schema WHERE type = 'view' AND name = 'logbook'",
	).Scan(&logbookSQL); err != nil {
		t.Fatalf("read logbook SQL: %v", err)
	}
	if strings.Contains(strings.ToUpper(logbookSQL), "ORDER BY") {
		t.Errorf("logbook SQL = %q, want no ORDER BY", logbookSQL)
	}

	var taskEntry struct {
		kind         string
		id           int64
		title        string
		status       string
		resolvedAt   string
		projectTitle *string
	}
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT kind, id, title, status, resolved_at, project_title FROM logbook WHERE kind = 'task' AND id = ?",
		contained.ID,
	).Scan(
		&taskEntry.kind,
		&taskEntry.id,
		&taskEntry.title,
		&taskEntry.status,
		&taskEntry.resolvedAt,
		&taskEntry.projectTitle,
	); err != nil {
		t.Fatalf("query task logbook entry: %v", err)
	}
	if taskEntry.kind != "task" || taskEntry.id != contained.ID || taskEntry.title != contained.Title ||
		taskEntry.status != "cancelled" || taskEntry.resolvedAt != taskResolvedAt ||
		taskEntry.projectTitle == nil || *taskEntry.projectTitle != parent.Title {
		t.Errorf("task logbook entry = %#v, want resolved task with project title", taskEntry)
	}

	var projectKind, projectStatus, resolvedAt string
	var noProjectTitle *string
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT kind, status, resolved_at, project_title FROM logbook WHERE kind = 'project' AND id = ?",
		parent.ID,
	).Scan(&projectKind, &projectStatus, &resolvedAt, &noProjectTitle); err != nil {
		t.Fatalf("query project logbook entry: %v", err)
	}
	if projectKind != "project" || projectStatus != "done" || resolvedAt != projectResolvedAt || noProjectTitle != nil {
		t.Errorf(
			"project logbook entry = (%q, %q, %q, %#v), want done project without project title",
			projectKind,
			projectStatus,
			resolvedAt,
			noProjectTitle,
		)
	}
}

func schemaColumnNames(t *testing.T, database *sql.DB, objectName string) []string {
	t.Helper()

	rows, err := database.Query("SELECT name FROM pragma_table_xinfo(?) ORDER BY cid", objectName)
	if err != nil {
		t.Fatalf("query %s columns: %v", objectName, err)
	}
	defer func() { _ = rows.Close() }()

	columns := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan %s column: %v", objectName, err)
		}
		columns = append(columns, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s columns: %v", objectName, err)
	}

	return columns
}
