package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMilestoneFourSchemaColumnsConstraintsAndIndexes(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	columns := map[string][]string{
		"areas": {
			"id", "title", "note", "archived_at", "position", "created_at", "updated_at",
		},
		"projects": {
			"id", "area_id", "title", "note", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at",
		},
		"tasks": {
			"id", "project_id", "area_id", "title", "note", "defer_until", "due_on", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at",
		},
	}
	for object, want := range columns {
		if got := schemaColumnNames(t, storage.database, object); !reflect.DeepEqual(got, want) {
			t.Errorf("%s columns = %v, want %v", object, got, want)
		}
	}

	for tableName, indexName := range map[string]string{
		"projects": "idx_projects_area",
		"tasks":    "idx_tasks_area",
	} {
		var count int
		if err := storage.database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pragma_index_list(?)
WHERE name = ?
`, tableName, indexName).Scan(&count); err != nil {
			t.Fatalf("inspect %s: %v", indexName, err)
		}
		if count != 1 {
			t.Errorf("%s count = %d, want 1", indexName, count)
		}
	}
	var projectIndexCount int
	if err := storage.database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pragma_index_list('tasks')
WHERE name = 'idx_tasks_project'
`).Scan(&projectIndexCount); err != nil {
		t.Fatalf("inspect idx_tasks_project: %v", err)
	}
	if projectIndexCount != 1 {
		t.Errorf("idx_tasks_project count = %d, want 1", projectIndexCount)
	}

	for _, relationship := range []struct {
		table  string
		parent string
		column string
	}{
		{table: "projects", parent: "areas", column: "area_id"},
		{table: "tasks", parent: "projects", column: "project_id"},
		{table: "tasks", parent: "areas", column: "area_id"},
	} {
		var count int
		if err := storage.database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM pragma_foreign_key_list(?)
WHERE "table" = ? AND "from" = ? AND on_delete = 'RESTRICT'
`, relationship.table, relationship.parent, relationship.column).Scan(&count); err != nil {
			t.Fatalf("inspect %s.%s foreign key: %v", relationship.table, relationship.column, err)
		}
		if count != 1 {
			t.Errorf("%s.%s RESTRICT foreign key count = %d, want 1", relationship.table, relationship.column, count)
		}
	}

	for _, statement := range []string{
		"INSERT INTO areas (position) VALUES (0)",
		"INSERT INTO areas (title) VALUES ('missing position')",
	} {
		if _, err := storage.database.ExecContext(ctx, statement); err == nil {
			t.Errorf("%s error = nil, want required-column failure", statement)
		}
	}

	areaResult, err := storage.database.ExecContext(ctx, `
INSERT INTO areas (title, position) VALUES ('Home', 0)
`)
	if err != nil {
		t.Fatalf("insert area: %v", err)
	}
	areaID, err := areaResult.LastInsertId()
	if err != nil {
		t.Fatalf("read area ID: %v", err)
	}
	projectResult, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (title, position) VALUES ('Standalone', 0)
`)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, err := projectResult.LastInsertId()
	if err != nil {
		t.Fatalf("read project ID: %v", err)
	}

	for description, statement := range map[string]string{
		"project area FK": "INSERT INTO projects (area_id, title, position) VALUES (99, 'orphan', 0)",
		"task project FK": "INSERT INTO tasks (project_id, title, position) VALUES (99, 'orphan', 0)",
		"task area FK":    "INSERT INTO tasks (area_id, title, position) VALUES (99, 'orphan', 0)",
	} {
		if _, err := storage.database.ExecContext(ctx, statement); err == nil {
			t.Errorf("%s error = nil, want FK failure", description)
		}
	}
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO tasks (project_id, area_id, title, position)
VALUES (?, ?, 'two containers', 0)
`, projectID, areaID); err == nil {
		t.Error("insert task with project and area error = nil, want CHECK failure")
	}

	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (area_id, title, position) VALUES (?, 'Contained project', 0)
`, areaID); err != nil {
		t.Fatalf("insert contained project: %v", err)
	}
	taskAreaID := insertFixture(t, storage.database, `
INSERT INTO areas (title, position) VALUES ('Task area', 1)
`)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO tasks (area_id, title, position) VALUES (?, 'Loose task', 0)
`, taskAreaID); err != nil {
		t.Fatalf("insert loose area task: %v", err)
	}
	for description, id := range map[string]int64{
		"contained project": areaID,
		"loose task":        taskAreaID,
	} {
		if _, err := storage.database.ExecContext(ctx, "DELETE FROM areas WHERE id = ?", id); err == nil {
			t.Errorf("delete area with %s error = nil, want RESTRICT failure", description)
		}
	}
}

func TestAutomaticallyAllocatedEntityIDsAreNotReusedAfterDeletion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	first := make(map[string]int64, 3)
	for _, entity := range []struct {
		name   string
		insert string
	}{
		{name: "area", insert: "INSERT INTO areas (title, position) VALUES ('first', 0)"},
		{name: "project", insert: "INSERT INTO projects (title, position) VALUES ('first', 0)"},
		{name: "task", insert: "INSERT INTO tasks (title, position) VALUES ('first', 0)"},
	} {
		result, err := storage.database.ExecContext(ctx, entity.insert)
		if err != nil {
			t.Fatalf("insert first %s: %v", entity.name, err)
		}
		first[entity.name], err = result.LastInsertId()
		if err != nil {
			t.Fatalf("read first %s ID: %v", entity.name, err)
		}
	}
	for _, entity := range []struct {
		name  string
		table string
	}{
		{name: "task", table: "tasks"},
		{name: "project", table: "projects"},
		{name: "area", table: "areas"},
	} {
		if _, err := storage.database.ExecContext(
			ctx,
			"DELETE FROM "+entity.table+" WHERE id = ?",
			first[entity.name],
		); err != nil {
			t.Fatalf("delete first %s: %v", entity.name, err)
		}
	}

	for _, entity := range []struct {
		name   string
		insert string
	}{
		{name: "area", insert: "INSERT INTO areas (title, position) VALUES ('second', 0)"},
		{name: "project", insert: "INSERT INTO projects (title, position) VALUES ('second', 0)"},
		{name: "task", insert: "INSERT INTO tasks (title, position) VALUES ('second', 0)"},
	} {
		result, err := storage.database.ExecContext(ctx, entity.insert)
		if err != nil {
			t.Fatalf("insert second %s: %v", entity.name, err)
		}
		second, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("read second %s ID: %v", entity.name, err)
		}
		if second <= first[entity.name] {
			t.Errorf("%s ID after deletion = %d, want greater than %d", entity.name, second, first[entity.name])
		}
	}
}

func TestMilestoneFourViewsApplyAreaPredicatesAndExposeEnrichment(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	activeAreaID := insertFixture(t, storage.database, `
INSERT INTO areas (title, position) VALUES ('Home', 0)
`)
	archivedAreaID := insertFixture(t, storage.database, `
INSERT INTO areas (title, archived_at, position)
VALUES ('Retired', '2026-01-01T00:00:00.000Z', 1)
`)
	activeProjectID := insertFixture(t, storage.database, `
INSERT INTO projects (area_id, title, position) VALUES (?, 'Kitchen', 0)
`, activeAreaID)
	archivedProjectID := insertFixture(t, storage.database, `
INSERT INTO projects (area_id, title, position) VALUES (?, 'Old project', 0)
`, archivedAreaID)
	resolvedProjectID := insertFixture(t, storage.database, `
INSERT INTO projects (area_id, title, done_at, position)
VALUES (?, 'Finished project', '2026-01-02T00:00:00.000Z', 1)
`, activeAreaID)

	inboxID := insertFixture(t, storage.database, `
INSERT INTO tasks (title, position) VALUES ('Inbox', 0)
`)
	activeLooseID := insertFixture(t, storage.database, `
INSERT INTO tasks (area_id, title, position) VALUES (?, 'Active loose', 0)
`, activeAreaID)
	_ = insertFixture(t, storage.database, `
INSERT INTO tasks (area_id, title, position) VALUES (?, 'Archived loose', 0)
`, archivedAreaID)
	activeProjectTaskID := insertFixture(t, storage.database, `
INSERT INTO tasks (project_id, title, position) VALUES (?, 'Project task', 0)
`, activeProjectID)
	_ = insertFixture(t, storage.database, `
INSERT INTO tasks (project_id, title, position) VALUES (?, 'Archived project task', 0)
`, archivedProjectID)
	_ = insertFixture(t, storage.database, `
INSERT INTO tasks (project_id, title, position) VALUES (?, 'Resolved project task', 0)
`, resolvedProjectID)
	doneLooseID := insertFixture(t, storage.database, `
INSERT INTO tasks (area_id, title, done_at, position)
VALUES (?, 'Done loose', '2026-01-03T00:00:00.000Z', 1)
`, activeAreaID)
	cancelledProjectTaskID := insertFixture(t, storage.database, `
INSERT INTO tasks (project_id, title, cancelled_at, position)
VALUES (?, 'Cancelled project task', '2026-01-04T00:00:00.000Z', 1)
`, activeProjectID)

	wantTaskViewColumns := []string{
		"id", "project_id", "area_id", "title", "note", "defer_until", "due_on", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at", "project_title", "governing_area_id", "governing_area_title",
	}
	for _, view := range []string{"inbox", "available"} {
		if got := schemaColumnNames(t, storage.database, view); !reflect.DeepEqual(got, wantTaskViewColumns) {
			t.Errorf("%s columns = %v, want %v", view, got, wantTaskViewColumns)
		}
	}

	inboxRows := queryViewFixtures(t, storage.database, `
SELECT id, project_title, governing_area_id, governing_area_title
FROM inbox
ORDER BY id
`)
	if len(inboxRows) != 1 || inboxRows[0].id != inboxID ||
		inboxRows[0].projectTitle != nil || inboxRows[0].governingAreaID != nil ||
		inboxRows[0].governingAreaTitle != nil {
		t.Errorf("inbox rows = %#v, want only unenriched inbox task %d", inboxRows, inboxID)
	}

	availableRows := queryViewFixtures(t, storage.database, `
SELECT id, project_title, governing_area_id, governing_area_title
FROM available
ORDER BY id
`)
	if len(availableRows) != 3 {
		t.Fatalf("available rows = %#v, want inbox and two active-area paths", availableRows)
	}
	assertViewFixture(t, availableRows[0], inboxID, nil, nil, nil)
	assertViewFixture(t, availableRows[1], activeLooseID, nil, &activeAreaID, stringPointer("Home"))
	assertViewFixture(t, availableRows[2], activeProjectTaskID, stringPointer("Kitchen"), &activeAreaID, stringPointer("Home"))

	wantLogbookColumns := []string{
		"kind", "id", "title", "status", "resolved_at", "project_title", "governing_area_id", "governing_area_title",
	}
	if got := schemaColumnNames(t, storage.database, "logbook"); !reflect.DeepEqual(got, wantLogbookColumns) {
		t.Errorf("logbook columns = %v, want %v", got, wantLogbookColumns)
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

	rows, err := storage.database.QueryContext(ctx, `
SELECT kind, id, project_title, governing_area_id, governing_area_title
FROM logbook
ORDER BY kind, id
`)
	if err != nil {
		t.Fatalf("query logbook enrichment: %v", err)
	}
	defer func() { _ = rows.Close() }()

	type logbookFixture struct {
		kind               string
		id                 int64
		projectTitle       *string
		governingAreaID    *int64
		governingAreaTitle *string
	}
	entries := make([]logbookFixture, 0, 3)
	for rows.Next() {
		var entry logbookFixture
		if err := rows.Scan(
			&entry.kind,
			&entry.id,
			&entry.projectTitle,
			&entry.governingAreaID,
			&entry.governingAreaTitle,
		); err != nil {
			t.Fatalf("scan logbook enrichment: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate logbook enrichment: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("logbook entries = %#v, want resolved project and two resolved tasks", entries)
	}
	if entries[0].kind != "project" || entries[0].id != resolvedProjectID ||
		entries[0].projectTitle != nil || !sameInt64Pointer(entries[0].governingAreaID, activeAreaID) ||
		!sameStringPointer(entries[0].governingAreaTitle, "Home") {
		t.Errorf("project logbook entry = %#v, want active-area enrichment", entries[0])
	}
	if entries[1].kind != "task" || entries[1].id != doneLooseID ||
		entries[1].projectTitle != nil || !sameInt64Pointer(entries[1].governingAreaID, activeAreaID) ||
		!sameStringPointer(entries[1].governingAreaTitle, "Home") {
		t.Errorf("direct task logbook entry = %#v, want active-area enrichment", entries[1])
	}
	if entries[2].kind != "task" || entries[2].id != cancelledProjectTaskID ||
		!sameStringPointer(entries[2].projectTitle, "Kitchen") ||
		!sameInt64Pointer(entries[2].governingAreaID, activeAreaID) ||
		!sameStringPointer(entries[2].governingAreaTitle, "Home") {
		t.Errorf("project task logbook entry = %#v, want inherited area enrichment", entries[2])
	}
}

type viewFixture struct {
	id                 int64
	projectTitle       *string
	governingAreaID    *int64
	governingAreaTitle *string
}

func insertFixture(t *testing.T, database *sql.DB, query string, arguments ...any) int64 {
	t.Helper()

	result, err := database.Exec(query, arguments...)
	if err != nil {
		t.Fatalf("insert fixture: %v", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read fixture ID: %v", err)
	}

	return id
}

func queryViewFixtures(t *testing.T, database *sql.DB, query string) []viewFixture {
	t.Helper()

	rows, err := database.Query(query)
	if err != nil {
		t.Fatalf("query view fixtures: %v", err)
	}
	defer func() { _ = rows.Close() }()

	fixtures := make([]viewFixture, 0)
	for rows.Next() {
		var fixture viewFixture
		if err := rows.Scan(
			&fixture.id,
			&fixture.projectTitle,
			&fixture.governingAreaID,
			&fixture.governingAreaTitle,
		); err != nil {
			t.Fatalf("scan view fixture: %v", err)
		}
		fixtures = append(fixtures, fixture)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate view fixtures: %v", err)
	}

	return fixtures
}

func assertViewFixture(
	t *testing.T,
	got viewFixture,
	wantID int64,
	wantProjectTitle *string,
	wantAreaID *int64,
	wantAreaTitle *string,
) {
	t.Helper()

	if got.id != wantID || !reflect.DeepEqual(got.projectTitle, wantProjectTitle) ||
		!reflect.DeepEqual(got.governingAreaID, wantAreaID) ||
		!reflect.DeepEqual(got.governingAreaTitle, wantAreaTitle) {
		t.Errorf(
			"view fixture = %#v, want ID %d/project %#v/area %#v/title %#v",
			got,
			wantID,
			wantProjectTitle,
			wantAreaID,
			wantAreaTitle,
		)
	}
}

func stringPointer(value string) *string {
	return &value
}

func sameInt64Pointer(got *int64, want int64) bool {
	return got != nil && *got == want
}

func sameStringPointer(got *string, want string) bool {
	return got != nil && *got == want
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
