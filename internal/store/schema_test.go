package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMilestoneFiveSchemaColumnsConstraintsAndIndexes(t *testing.T) {
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
		"tags":         {"id", "title", "created_at", "updated_at"},
		"task_tags":    {"task_id", "tag_id"},
		"project_tags": {"project_id", "tag_id"},
		"area_tags":    {"area_id", "tag_id"},
	}
	for object, want := range columns {
		if got := schemaColumnNames(t, storage.database, object); !reflect.DeepEqual(got, want) {
			t.Errorf("%s columns = %v, want %v", object, got, want)
		}
	}

	for tableName, indexNames := range map[string][]string{
		"projects":     {"idx_projects_area"},
		"tasks":        {"idx_tasks_project", "idx_tasks_area"},
		"task_tags":    {"idx_task_tags_tag"},
		"project_tags": {"idx_project_tags_tag"},
		"area_tags":    {"idx_area_tags_tag"},
	} {
		for _, indexName := range indexNames {
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

	areaID := insertFixture(t, storage.database, `
INSERT INTO areas (title, position) VALUES ('Home', 0)
`)
	projectID := insertFixture(t, storage.database, `
INSERT INTO projects (title, position) VALUES ('Standalone', 0)
`)

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

func TestTagJoinTablesEnforceIdentityRelationshipsAndStorageShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	for table, wantPK := range map[string][]string{
		"task_tags":    {"task_id", "tag_id"},
		"project_tags": {"project_id", "tag_id"},
		"area_tags":    {"area_id", "tag_id"},
	} {
		var strict, withoutRowID int
		if err := storage.database.QueryRowContext(ctx, `
SELECT strict, wr FROM pragma_table_list WHERE name = ?
`, table).Scan(&strict, &withoutRowID); err != nil {
			t.Fatalf("inspect %s storage: %v", table, err)
		}
		if strict != 1 || withoutRowID != 1 {
			t.Errorf("%s strict/without-rowid = %d/%d, want 1/1", table, strict, withoutRowID)
		}

		rows, err := storage.database.QueryContext(ctx, `
SELECT name FROM pragma_table_info(?) WHERE pk > 0 ORDER BY pk
`, table)
		if err != nil {
			t.Fatalf("inspect %s primary key: %v", table, err)
		}
		gotPK := make([]string, 0, 2)
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				_ = rows.Close()
				t.Fatalf("scan %s primary key: %v", table, err)
			}
			gotPK = append(gotPK, column)
		}
		if err := rows.Close(); err != nil {
			t.Fatalf("close %s primary key rows: %v", table, err)
		}
		if !reflect.DeepEqual(gotPK, wantPK) {
			t.Errorf("%s primary key = %v, want %v", table, gotPK, wantPK)
		}

		var cascadeCount int
		if err := storage.database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM pragma_foreign_key_list(?) WHERE on_delete = 'CASCADE'
`, table).Scan(&cascadeCount); err != nil {
			t.Fatalf("inspect %s foreign keys: %v", table, err)
		}
		if cascadeCount != 2 {
			t.Errorf("%s CASCADE foreign keys = %d, want 2", table, cascadeCount)
		}
	}

	for indexName := range map[string]struct{}{
		"idx_task_tags_tag":    {},
		"idx_project_tags_tag": {},
		"idx_area_tags_tag":    {},
	} {
		var indexedColumn string
		if err := storage.database.QueryRowContext(ctx, `
SELECT name FROM pragma_index_info(?) WHERE seqno = 0
`, indexName).Scan(&indexedColumn); err != nil {
			t.Fatalf("inspect %s columns: %v", indexName, err)
		}
		if indexedColumn != "tag_id" {
			t.Errorf("%s first column = %q, want tag_id", indexName, indexedColumn)
		}
	}

	if _, err := storage.database.ExecContext(ctx, "INSERT INTO tags DEFAULT VALUES"); err == nil {
		t.Error("insert tag without title error = nil, want required-column failure")
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO tags (id, title) VALUES ('wrong', 'typed')"); err == nil {
		t.Error("insert non-integer tag ID error = nil, want STRICT failure")
	}
	tagID := insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('Errands')")
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO tags (title) VALUES ('errands')"); err == nil {
		t.Error("insert case-duplicate tag error = nil, want NOCASE uniqueness failure")
	}
	taskID := insertFixture(t, storage.database, "INSERT INTO tasks (title, position) VALUES ('task', 0)")
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)", taskID, tagID); err != nil {
		t.Fatalf("insert task tag: %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)", taskID, tagID); err == nil {
		t.Error("insert duplicate task tag error = nil, want composite-PK failure")
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO task_tags (task_id, tag_id) VALUES (999, ?)", tagID); err == nil {
		t.Error("insert tag for missing task error = nil, want FK failure")
	}
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO task_tags (task_id, tag_id) VALUES (?, 999)", taskID); err == nil {
		t.Error("insert missing tag error = nil, want FK failure")
	}
}

func TestTagJoinRowsCascadeFromTagsAndEveryEntityKind(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	seedAttachments := func(tagTitle, suffix string) (int64, map[string]int64) {
		t.Helper()
		tagID := insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES (?)", tagTitle)
		ids := map[string]int64{
			"task":    insertFixture(t, storage.database, "INSERT INTO tasks (title, position) VALUES (?, 0)", "task "+suffix),
			"project": insertFixture(t, storage.database, "INSERT INTO projects (title, position) VALUES (?, 0)", "project "+suffix),
			"area":    insertFixture(t, storage.database, "INSERT INTO areas (title, position) VALUES (?, 0)", "area "+suffix),
		}
		for _, fixture := range []struct {
			table  string
			column string
			id     int64
		}{
			{table: "task_tags", column: "task_id", id: ids["task"]},
			{table: "project_tags", column: "project_id", id: ids["project"]},
			{table: "area_tags", column: "area_id", id: ids["area"]},
		} {
			if _, err := storage.database.ExecContext(
				ctx,
				"INSERT INTO "+fixture.table+" ("+fixture.column+", tag_id) VALUES (?, ?)",
				fixture.id,
				tagID,
			); err != nil {
				t.Fatalf("attach %s: %v", fixture.table, err)
			}
		}
		return tagID, ids
	}

	firstTagID, firstEntities := seedAttachments("first", "first")
	if _, err := storage.database.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", firstTagID); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	for _, table := range []string{"task_tags", "project_tags", "area_tags"} {
		if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM "+table); count != 0 {
			t.Errorf("%s rows after tag deletion = %d, want 0", table, count)
		}
	}
	for table, id := range firstEntities {
		if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM "+table+"s WHERE id = ?", id); count != 1 {
			t.Errorf("%s %d after tag deletion count = %d, want 1", table, id, count)
		}
	}

	secondTagID, secondEntities := seedAttachments("second", "second")
	for table, id := range secondEntities {
		if _, err := storage.database.ExecContext(ctx, "DELETE FROM "+table+"s WHERE id = ?", id); err != nil {
			t.Fatalf("delete %s: %v", table, err)
		}
	}
	for _, table := range []string{"task_tags", "project_tags", "area_tags"} {
		if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM "+table); count != 0 {
			t.Errorf("%s rows after entity deletion = %d, want 0", table, count)
		}
	}
	if count := fixtureCount(t, storage.database, "SELECT COUNT(*) FROM tags WHERE id = ?", secondTagID); count != 1 {
		t.Errorf("tag after entity deletion count = %d, want 1", count)
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

	entities := []string{"areas", "projects", "tasks"}
	firstIDs := make([]int64, len(entities))
	for index, tableName := range entities {
		firstIDs[index] = insertFixture(
			t,
			storage.database,
			"INSERT INTO "+tableName+" (title, position) VALUES ('first', 0)",
		)
	}
	for index, tableName := range entities {
		if _, err := storage.database.ExecContext(
			ctx,
			"DELETE FROM "+tableName+" WHERE id = ?",
			firstIDs[index],
		); err != nil {
			t.Fatalf("delete first row from %s: %v", tableName, err)
		}
	}

	for index, tableName := range entities {
		secondID := insertFixture(
			t,
			storage.database,
			"INSERT INTO "+tableName+" (title, position) VALUES ('second', 0)",
		)
		if secondID <= firstIDs[index] {
			t.Errorf(
				"%s ID after deletion = %d, want greater than %d",
				tableName,
				secondID,
				firstIDs[index],
			)
		}
	}

	firstTagID := insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('first')")
	if _, err := storage.database.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", firstTagID); err != nil {
		t.Fatalf("delete first tag: %v", err)
	}
	secondTagID := insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('second')")
	if secondTagID <= firstTagID {
		t.Errorf("tag ID after deletion = %d, want greater than %d", secondTagID, firstTagID)
	}
}

func TestMilestoneFiveViewsExposeTagsInCreationOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	tagIDs := []int64{
		insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('Zulu')"),
		insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('alpha')"),
		insertFixture(t, storage.database, "INSERT INTO tags (title) VALUES ('Middle')"),
	}
	inboxTaskID := insertFixture(t, storage.database, "INSERT INTO tasks (title, position) VALUES ('inbox', 0)")
	_ = insertFixture(t, storage.database, "INSERT INTO tasks (title, position) VALUES ('untagged', 1)")
	resolvedTaskID := insertFixture(t, storage.database, `
INSERT INTO tasks (title, done_at, position) VALUES ('resolved task', '2026-01-01T00:00:00.000Z', 2)
`)
	resolvedProjectID := insertFixture(t, storage.database, `
INSERT INTO projects (title, cancelled_at, position) VALUES ('resolved project', '2026-01-02T00:00:00.000Z', 0)
`)
	for _, attachment := range []struct {
		table  string
		column string
		id     int64
		tagID  int64
	}{
		{table: "task_tags", column: "task_id", id: inboxTaskID, tagID: tagIDs[2]},
		{table: "task_tags", column: "task_id", id: inboxTaskID, tagID: tagIDs[0]},
		{table: "task_tags", column: "task_id", id: inboxTaskID, tagID: tagIDs[1]},
		{table: "task_tags", column: "task_id", id: resolvedTaskID, tagID: tagIDs[1]},
		{table: "task_tags", column: "task_id", id: resolvedTaskID, tagID: tagIDs[0]},
		{table: "project_tags", column: "project_id", id: resolvedProjectID, tagID: tagIDs[2]},
		{table: "project_tags", column: "project_id", id: resolvedProjectID, tagID: tagIDs[0]},
	} {
		if _, err := storage.database.ExecContext(
			ctx,
			"INSERT INTO "+attachment.table+" ("+attachment.column+", tag_id) VALUES (?, ?)",
			attachment.id,
			attachment.tagID,
		); err != nil {
			t.Fatalf("insert raw %s fixture: %v", attachment.table, err)
		}
	}

	for _, query := range []struct {
		name string
		sql  string
		args []any
		want string
	}{
		{name: "inbox", sql: "SELECT tags FROM inbox WHERE id = ?", args: []any{inboxTaskID}, want: `["Zulu","alpha","Middle"]`},
		{name: "available", sql: "SELECT tags FROM available WHERE id = ?", args: []any{inboxTaskID}, want: `["Zulu","alpha","Middle"]`},
		{name: "untagged", sql: "SELECT tags FROM inbox WHERE title = 'untagged'", want: `[]`},
		{name: "logbook task", sql: "SELECT tags FROM logbook WHERE kind = 'task' AND id = ?", args: []any{resolvedTaskID}, want: `["Zulu","alpha"]`},
		{name: "logbook project", sql: "SELECT tags FROM logbook WHERE kind = 'project' AND id = ?", args: []any{resolvedProjectID}, want: `["Zulu","Middle"]`},
	} {
		var got string
		if err := storage.database.QueryRowContext(ctx, query.sql, query.args...).Scan(&got); err != nil {
			t.Fatalf("query %s tags: %v", query.name, err)
		}
		if got != query.want {
			t.Errorf("%s tags = %s, want %s", query.name, got, query.want)
		}
	}

	wantViews := map[string]string{
		"inbox": `CREATE VIEW inbox AS
SELECT t.*,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.id)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.project_id IS NULL AND t.area_id IS NULL AND t.status = 'open'`,
		"available": `CREATE VIEW available AS
SELECT t.*,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.id)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status = 'open'
  AND (t.project_id IS NULL OR p.status = 'open')
  AND a.archived_at IS NULL
  AND (t.defer_until IS NULL OR t.defer_until <= date('now', 'localtime'))`,
		"logbook": `CREATE VIEW logbook AS
SELECT 'task' AS kind, t.id, t.title, t.status,
       COALESCE(t.done_at, t.cancelled_at) AS resolved_at,
       p.title                        AS project_title,
       COALESCE(t.area_id, p.area_id) AS governing_area_id,
       a.title                        AS governing_area_title,
       (SELECT json_group_array(g.title ORDER BY g.id)
        FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
        WHERE tt.task_id = t.id)      AS tags
FROM tasks t
LEFT JOIN projects p ON p.id = t.project_id
LEFT JOIN areas    a ON a.id = COALESCE(t.area_id, p.area_id)
WHERE t.status IN ('done', 'cancelled')
UNION ALL
SELECT 'project', p.id, p.title, p.status,
       COALESCE(p.done_at, p.cancelled_at),
       NULL,
       p.area_id,
       a.title,
       (SELECT json_group_array(g.title ORDER BY g.id)
        FROM project_tags pt JOIN tags g ON g.id = pt.tag_id
        WHERE pt.project_id = p.id)
FROM projects p
LEFT JOIN areas a ON a.id = p.area_id
WHERE p.status IN ('done', 'cancelled')`,
	}
	for name, want := range wantViews {
		var got string
		if err := storage.database.QueryRowContext(
			ctx,
			"SELECT sql FROM sqlite_schema WHERE type = 'view' AND name = ?",
			name,
		).Scan(&got); err != nil {
			t.Fatalf("read %s view SQL: %v", name, err)
		}
		if got != want {
			t.Errorf("%s view SQL = %q, want canonical definition %q", name, got, want)
		}
	}
}

func TestMilestoneFiveViewsApplyAreaPredicatesAndExposeEnrichment(t *testing.T) {
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
		"id", "project_id", "area_id", "title", "note", "defer_until", "due_on", "done_at", "cancelled_at", "status", "position", "created_at", "updated_at", "project_title", "governing_area_id", "governing_area_title", "tags",
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
		"kind", "id", "title", "status", "resolved_at", "project_title", "governing_area_id", "governing_area_title", "tags",
	}
	if got := schemaColumnNames(t, storage.database, "logbook"); !reflect.DeepEqual(got, wantLogbookColumns) {
		t.Errorf("logbook columns = %v, want %v", got, wantLogbookColumns)
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

func fixtureCount(t *testing.T, database *sql.DB, query string, arguments ...any) int {
	t.Helper()

	var count int
	if err := database.QueryRow(query, arguments...).Scan(&count); err != nil {
		t.Fatalf("count fixture: %v", err)
	}
	return count
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
