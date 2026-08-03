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

func TestOpenBootstrapsMilestoneSixSchemaAndConfiguresConnections(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "gsd.db")
	storage, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = storage.Close()
	})

	var version int
	if err := storage.database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 9006 {
		t.Errorf("user_version = %d, want 9006", version)
	}

	for _, tableName := range []string{
		"areas", "projects", "tasks", "tags", "task_tags", "project_tags", "area_tags",
	} {
		var strict int
		if err := storage.database.QueryRowContext(
			ctx,
			"SELECT strict FROM pragma_table_list WHERE name = ?",
			tableName,
		).Scan(&strict); err != nil {
			t.Fatalf("read %s table metadata: %v", tableName, err)
		}
		if strict != 1 {
			t.Errorf("%s strict = %d, want 1", tableName, strict)
		}
	}

	storage.database.SetMaxOpenConns(2)
	first, err := storage.database.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire first connection: %v", err)
	}
	defer func() {
		_ = first.Close()
	}()
	second, err := storage.database.Conn(ctx)
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	defer func() {
		_ = second.Close()
	}()

	for index, connection := range []*sql.Conn{first, second} {
		var foreignKeys int
		var busyTimeout int
		if err := connection.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("connection %d foreign_keys: %v", index, err)
		}
		if err := connection.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("connection %d busy_timeout: %v", index, err)
		}
		if foreignKeys != 1 {
			t.Errorf("connection %d foreign_keys = %d, want 1", index, foreignKeys)
		}
		if busyTimeout != busyTimeoutMS {
			t.Errorf("connection %d busy_timeout = %d, want %d", index, busyTimeout, busyTimeoutMS)
		}
	}

	var journalMode string
	if err := first.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if strings.EqualFold(journalMode, "wal") {
		t.Errorf("journal_mode = %q, want non-WAL mode", journalMode)
	}
}

func TestContainedListingsDoNotReserveWriter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gsd.db")
	reader, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(reader) error = %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	areas := NewAreas(reader)
	projects := NewProjects(reader)
	tasks := NewTasks(reader)
	container := addStoredArea(t, areas, area.AddFields{Title: "area"})
	contained := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "project"})
	created := addStoredTask(t, tasks, task.AddFields{ProjectID: &contained.ID, Title: "task"})
	if _, err := reader.database.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		t.Fatalf("disable reader busy timeout: %v", err)
	}

	writer, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("Open(writer) error = %v", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	writerConnection, err := writer.database.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve writer connection: %v", err)
	}
	defer func() { _ = writerConnection.Close() }()
	if _, err := writerConnection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("reserve writer transaction: %v", err)
	}
	defer func() { _, _ = writerConnection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK") }()

	listedProjects, err := projects.List(
		ctx,
		project.ListOptions{Status: project.ListStatusAll, AreaID: &container.ID},
	)
	if err != nil {
		t.Fatalf("List(projects while writer reserved) error = %v", err)
	}
	if len(listedProjects) != 1 || listedProjects[0].ID != contained.ID {
		t.Errorf("List(projects while writer reserved) = %#v, want project %d", listedProjects, contained.ID)
	}
	listedTasks, err := tasks.List(
		ctx,
		task.ListFilter{Status: task.ListStatusAll, ProjectID: &contained.ID},
	)
	if err != nil {
		t.Fatalf("List(tasks while writer reserved) error = %v", err)
	}
	if len(listedTasks) != 1 || listedTasks[0].ID != created.ID {
		t.Errorf("List(tasks while writer reserved) = %#v, want task %d", listedTasks, created.ID)
	}
}

func TestDateColumnsEnforceCanonicalValuesAndRoundTripDates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	dueOn := "2026-08-03"
	deferUntil := "2026-08-01"
	created, err := tasks.Add(ctx, task.AddFields{
		Title:      "dated",
		DueOn:      &dueOn,
		DeferUntil: &deferUntil,
	}, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if created.DueOn == nil || *created.DueOn != dueOn ||
		created.DeferUntil == nil || *created.DeferUntil != deferUntil {
		t.Errorf("Add() dates = %#v/%#v, want canonical due and defer", created.DueOn, created.DeferUntil)
	}

	for _, test := range []struct {
		name   string
		column string
		value  string
	}{
		{name: "invalid defer", column: "defer_until", value: "2026-02-30"},
		{name: "noncanonical defer", column: "defer_until", value: "2026-8-3"},
		{name: "invalid due", column: "due_on", value: "2026-02-30"},
		{name: "noncanonical due", column: "due_on", value: "2026-8-3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, insertErr := storage.database.ExecContext(
				ctx,
				"INSERT INTO tasks (title, "+test.column+", position) VALUES (?, ?, ?)",
				test.name,
				test.value,
				created.Position+1,
			)
			if insertErr == nil {
				t.Fatalf("insert %s = nil, want CHECK failure", test.value)
			}
		})
	}
}

func TestAvailableViewUsesLocalDeferBoundaryAndOpenStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for attempt := range 3 {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("attempt-%d.db", attempt))
		storage, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		tasks := NewTasks(storage)

		var localDate string
		if err := storage.database.QueryRowContext(ctx, "SELECT date('now', 'localtime')").Scan(&localDate); err != nil {
			_ = storage.Close()
			t.Fatalf("capture SQLite local date: %v", err)
		}

		created := make([]task.Task, 5)
		for index, title := range []string{"null", "past", "today", "future", "done"} {
			created[index], err = tasks.Add(ctx, task.AddFields{Title: title}, "2026-01-01T00:00:00.000Z")
			if err != nil {
				_ = storage.Close()
				t.Fatalf("Add(%s) error = %v", title, err)
			}
		}
		deferDates := map[int]string{
			1: "0000-01-01",
			2: localDate,
			3: "9999-12-31",
			4: localDate,
		}
		for index, value := range deferDates {
			if _, err := storage.database.ExecContext(
				ctx,
				"UPDATE tasks SET defer_until = ? WHERE id = ?",
				value,
				created[index].ID,
			); err != nil {
				_ = storage.Close()
				t.Fatalf("set defer date: %v", err)
			}
		}
		if _, err := tasks.Done(ctx, created[4].ID, "2026-01-02T00:00:00.000Z"); err != nil {
			_ = storage.Close()
			t.Fatalf("Done() error = %v", err)
		}

		available, err := tasks.Available(ctx)
		if err != nil {
			_ = storage.Close()
			t.Fatalf("Available() error = %v", err)
		}
		got := make([]int64, len(available))
		for index := range available {
			got[index] = available[index].ID
		}

		var recheckedDate string
		if err := storage.database.QueryRowContext(ctx, "SELECT date('now', 'localtime')").Scan(&recheckedDate); err != nil {
			_ = storage.Close()
			t.Fatalf("recheck SQLite local date: %v", err)
		}
		if err := storage.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if localDate != recheckedDate {
			continue
		}

		want := []int64{created[0].ID, created[1].ID, created[2].ID}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("available IDs = %v, want null, past, and today IDs %v", got, want)
		}
		return
	}

	t.Fatal("SQLite local date rolled over during every available-view attempt")
}

func TestAddAppendsPositionsAcrossResolvedTasksAndGeneratesStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() {
		_ = storage.Close()
	})

	createdAt := "2026-01-01T00:00:00.000Z"
	first, err := tasks.Add(ctx, task.AddFields{Title: "first", Note: "first note"}, createdAt)
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if first.Status != "open" || first.Position != 0 {
		t.Errorf("first task = %#v, want open at position 0", first)
	}
	if first.CreatedAt != createdAt || first.UpdatedAt != createdAt {
		t.Errorf(
			"first timestamps = (%q, %q), want supplied %q on both",
			first.CreatedAt,
			first.UpdatedAt,
			createdAt,
		)
	}

	resolvedAt := "2026-01-02T00:00:00.000Z"
	if _, err := tasks.Done(ctx, first.ID, resolvedAt); err != nil {
		t.Fatalf("Done(first) error = %v", err)
	}

	second, err := tasks.Add(ctx, task.AddFields{Title: "second"}, "2026-01-03T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	if second.Position != 1 {
		t.Errorf("second position = %d, want appended past resolved first", second.Position)
	}

	inbox, err := tasks.Inbox(ctx)
	if err != nil {
		t.Fatalf("Inbox() error = %v", err)
	}
	if len(inbox) != 1 || inbox[0].ID != second.ID {
		t.Errorf("Inbox() = %#v, want only second task", inbox)
	}

	found, err := tasks.Find(ctx, first.ID)
	if err != nil {
		t.Fatalf("Find(first) error = %v", err)
	}
	if found.Status != "done" || found.DoneAt == nil || *found.DoneAt != resolvedAt {
		t.Errorf("Find(first) = %#v, want generated done status", found)
	}
}

func TestOpenRejectsUnsafeBootstrapStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		setup string
	}{
		{name: "wrong revision", setup: "PRAGMA user_version = 42"},
		{name: "nonempty version zero", setup: "CREATE TABLE existing (id INTEGER)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "gsd.db")
			database, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatalf("sql.Open() error = %v", err)
			}
			if _, err := database.Exec(test.setup); err != nil {
				_ = database.Close()
				t.Fatalf("prepare database: %v", err)
			}
			if err := database.Close(); err != nil {
				t.Fatalf("close prepared database: %v", err)
			}

			_, err = Open(context.Background(), path)
			if err == nil {
				t.Fatal("Open() error = nil, want conflict")
			}
			code, ok := apperr.CodeOf(err)
			if !ok || code != apperr.Conflict {
				t.Errorf("Open() error = %v, want conflict", err)
			}
			if !strings.Contains(err.Error(), "delete your development database") {
				t.Errorf("Open() error = %q, want delete guidance", err)
			}
		})
	}
}

func TestOpenAcceptsExistingCurrentRevision(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "gsd.db")
	storage, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("first Open() error = %v", err)
	}
	if err := storage.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("second Open() error = %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestConcurrentOpenBootstrapsOnce(t *testing.T) {
	t.Parallel()

	const openerCount = 4
	path := filepath.Join(t.TempDir(), "nested", "gsd.db")
	start := make(chan struct{})
	results := make(chan error, openerCount)
	for range openerCount {
		go func() {
			<-start
			storage, err := Open(context.Background(), path)
			if err == nil {
				err = storage.Close()
			}
			results <- err
		}()
	}
	close(start)

	for range openerCount {
		if err := <-results; err != nil {
			t.Errorf("concurrent Open() error = %v", err)
		}
	}
}

func TestFindReturnsNotFoundCode(t *testing.T) {
	t.Parallel()

	storage, err := Open(context.Background(), filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() {
		_ = storage.Close()
	})

	_, err = tasks.Find(context.Background(), 99)
	if err == nil {
		t.Fatal("Find() error = nil, want not_found")
	}
	code, ok := apperr.CodeOf(err)
	if !ok || code != apperr.NotFound {
		t.Errorf("Find() error = %v, want not_found", err)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("Find() error = %v, want wrapped sql.ErrNoRows", err)
	}
}

func TestListFiltersGeneratedStatusesAndOrdersByPositionThenID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	created := make([]task.Task, 4)
	for index, title := range []string{"first", "second", "third", "fourth"} {
		created[index], err = tasks.Add(ctx, task.AddFields{Title: title}, "2026-01-01T00:00:00.000Z")
		if err != nil {
			t.Fatalf("Add(%s) error = %v", title, err)
		}
	}
	if _, err := tasks.Done(ctx, created[1].ID, "2026-01-02T00:00:00.000Z"); err != nil {
		t.Fatalf("Done(second) error = %v", err)
	}
	if _, err := tasks.Cancel(ctx, created[2].ID, "2026-01-03T00:00:00.000Z"); err != nil {
		t.Fatalf("Cancel(third) error = %v", err)
	}
	if _, err := storage.database.ExecContext(ctx, `
UPDATE tasks
SET position = CASE id
    WHEN ? THEN 2
    WHEN ? THEN 1
    WHEN ? THEN 1
    WHEN ? THEN 0
END
`, created[0].ID, created[1].ID, created[2].ID, created[3].ID); err != nil {
		t.Fatalf("arrange positions: %v", err)
	}

	tests := []struct {
		status task.ListStatus
		want   []int64
	}{
		{status: task.ListStatusOpen, want: []int64{created[3].ID, created[0].ID}},
		{status: task.ListStatusDone, want: []int64{created[1].ID}},
		{status: task.ListStatusCancelled, want: []int64{created[2].ID}},
		{status: task.ListStatusAll, want: []int64{created[3].ID, created[1].ID, created[2].ID, created[0].ID}},
	}
	for _, test := range tests {
		listed, listErr := tasks.List(ctx, task.ListFilter{Status: test.status})
		if listErr != nil {
			t.Fatalf("List(%s) error = %v", test.status, listErr)
		}
		got := make([]int64, len(listed))
		for index := range listed {
			got[index] = listed[index].ID
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("List(%s) IDs = %v, want %v", test.status, got, test.want)
		}
	}
}

func TestListDatePredicatesComposeWithStatusAndPreserveOrdering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for attempt := range 3 {
		storage, err := Open(ctx, filepath.Join(t.TempDir(), fmt.Sprintf("attempt-%d.db", attempt)))
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		tasks := NewTasks(storage)

		var localDate string
		if err := storage.database.QueryRowContext(ctx, "SELECT date('now', 'localtime')").Scan(&localDate); err != nil {
			_ = storage.Close()
			t.Fatalf("capture SQLite local date: %v", err)
		}
		past := "0000-01-01"
		future := "9999-12-31"
		fields := []task.AddFields{
			{Title: "undated"},
			{Title: "past", DueOn: &past, DeferUntil: &future},
			{Title: "today", DueOn: &localDate, DeferUntil: &localDate},
			{Title: "future", DueOn: &future, DeferUntil: &past},
			{Title: "done past", DueOn: &past, DeferUntil: &future},
		}
		created := make([]task.Task, len(fields))
		for index := range fields {
			created[index], err = tasks.Add(ctx, fields[index], "2026-01-01T00:00:00.000Z")
			if err != nil {
				_ = storage.Close()
				t.Fatalf("Add(%s) error = %v", fields[index].Title, err)
			}
		}
		if _, err := tasks.Done(ctx, created[4].ID, "2026-01-02T00:00:00.000Z"); err != nil {
			_ = storage.Close()
			t.Fatalf("Done() error = %v", err)
		}

		mismatches := make([]string, 0)
		tests := []struct {
			name   string
			filter task.ListFilter
			want   []int64
		}{
			{
				name:   "open due",
				filter: task.ListFilter{Status: task.ListStatusOpen, Date: task.DateSelectorDue},
				want:   []int64{created[1].ID, created[2].ID, created[3].ID},
			},
			{
				name:   "all due",
				filter: task.ListFilter{Status: task.ListStatusAll, Date: task.DateSelectorDue},
				want:   []int64{created[1].ID, created[2].ID, created[3].ID, created[4].ID},
			},
			{
				name:   "done due",
				filter: task.ListFilter{Status: task.ListStatusDone, Date: task.DateSelectorDue},
				want:   []int64{created[4].ID},
			},
			{
				name:   "open overdue",
				filter: task.ListFilter{Status: task.ListStatusOpen, Date: task.DateSelectorOverdue},
				want:   []int64{created[1].ID},
			},
			{
				name:   "done overdue",
				filter: task.ListFilter{Status: task.ListStatusDone, Date: task.DateSelectorOverdue},
				want:   []int64{},
			},
			{
				name:   "all overdue",
				filter: task.ListFilter{Status: task.ListStatusAll, Date: task.DateSelectorOverdue},
				want:   []int64{created[1].ID},
			},
			{
				name:   "open deferred",
				filter: task.ListFilter{Status: task.ListStatusOpen, Date: task.DateSelectorDeferred},
				want:   []int64{created[1].ID},
			},
			{
				name:   "done deferred",
				filter: task.ListFilter{Status: task.ListStatusDone, Date: task.DateSelectorDeferred},
				want:   []int64{created[4].ID},
			},
			{
				name:   "all deferred",
				filter: task.ListFilter{Status: task.ListStatusAll, Date: task.DateSelectorDeferred},
				want:   []int64{created[1].ID, created[4].ID},
			},
		}
		for _, test := range tests {
			listed, listErr := tasks.List(ctx, test.filter)
			if listErr != nil {
				_ = storage.Close()
				t.Fatalf("List(%s) error = %v", test.name, listErr)
			}
			got := make([]int64, len(listed))
			for index := range listed {
				got[index] = listed[index].ID
			}
			if !reflect.DeepEqual(got, test.want) {
				mismatches = append(
					mismatches,
					fmt.Sprintf("List(%s) IDs = %v, want %v", test.name, got, test.want),
				)
			}
		}

		var recheckedDate string
		if err := storage.database.QueryRowContext(ctx, "SELECT date('now', 'localtime')").Scan(&recheckedDate); err != nil {
			_ = storage.Close()
			t.Fatalf("recheck SQLite local date: %v", err)
		}
		if err := storage.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if localDate != recheckedDate {
			continue
		}
		for _, mismatch := range mismatches {
			t.Error(mismatch)
		}
		return
	}

	t.Fatal("SQLite local date rolled over during every deadline-filter attempt")
}

func TestEditAtomicallyUpdatesRequestedFieldsAndReturnsTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	initialDueOn := "2026-08-02"
	initialDeferUntil := "2026-08-01"
	created, err := tasks.Add(
		ctx,
		task.AddFields{
			Title:      "original",
			Note:       "original note",
			DueOn:      &initialDueOn,
			DeferUntil: &initialDeferUntil,
		},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	done, err := tasks.Done(ctx, created.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}

	title := "  revised  "
	titleEdited, err := tasks.Edit(
		ctx,
		created.ID,
		task.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(title) error = %v", err)
	}
	if titleEdited.Title != title || titleEdited.Note != done.Note ||
		titleEdited.DueOn == nil || *titleEdited.DueOn != initialDueOn ||
		titleEdited.DeferUntil == nil || *titleEdited.DeferUntil != initialDeferUntil {
		t.Errorf("Edit(title) = %#v, want exact title and preserved note and dates", titleEdited)
	}
	if titleEdited.Status != done.Status || !reflect.DeepEqual(titleEdited.DoneAt, done.DoneAt) {
		t.Errorf("Edit(title) lifecycle = %#v, want preserved done state", titleEdited)
	}
	if titleEdited.Position != done.Position || titleEdited.CreatedAt != done.CreatedAt {
		t.Errorf("Edit(title) changed stable fields: %#v", titleEdited)
	}
	if titleEdited.UpdatedAt != "2026-01-03T00:00:00.000Z" {
		t.Errorf("Edit(title) updated_at = %q, want edit timestamp", titleEdited.UpdatedAt)
	}

	dueOn := "2026-08-03"
	deferUntil := "2026-08-02"
	dueEdited, err := tasks.Edit(
		ctx,
		created.ID,
		task.EditFields{
			DueOn:      task.DateChange{Set: &dueOn},
			DeferUntil: task.DateChange{Set: &deferUntil},
		},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(due) error = %v", err)
	}
	if dueEdited.DueOn == nil || *dueEdited.DueOn != dueOn ||
		dueEdited.DeferUntil == nil || *dueEdited.DeferUntil != deferUntil ||
		dueEdited.Title != title {
		t.Errorf("Edit(dates) = %#v, want updated dates and preserved title", dueEdited)
	}

	invalidTitle := "must not persist"
	invalidDueOn := "2026-02-30"
	if _, err := tasks.Edit(
		ctx,
		created.ID,
		task.EditFields{Title: &invalidTitle, DueOn: task.DateChange{Set: &invalidDueOn}},
		"2026-01-05T00:00:00.000Z",
	); err == nil {
		t.Fatal("Edit(invalid due) error = nil, want CHECK failure")
	}
	unchanged, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(after invalid edit) error = %v", err)
	}
	if !reflect.DeepEqual(unchanged, dueEdited) {
		t.Errorf("task after invalid edit = %#v, want unchanged %#v", unchanged, dueEdited)
	}

	note := ""
	noteEdited, err := tasks.Edit(
		ctx,
		created.ID,
		task.EditFields{
			Note:       &note,
			DueOn:      task.DateChange{Clear: true},
			DeferUntil: task.DateChange{Clear: true},
		},
		"2026-01-06T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(note) error = %v", err)
	}
	if noteEdited.Title != title || noteEdited.Note != "" ||
		noteEdited.DueOn != nil || noteEdited.DeferUntil != nil {
		t.Errorf("Edit(note and dates) = %#v, want preserved title and cleared note and dates", noteEdited)
	}
	persisted, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if !reflect.DeepEqual(persisted, noteEdited) {
		t.Errorf("Find() = %#v, want returned edit %#v", persisted, noteEdited)
	}
}

func TestEditRejectsNoFieldsAndReportsMissingTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	if _, err := tasks.Edit(ctx, 1, task.EditFields{}, "2026-01-01T00:00:00.000Z"); err == nil {
		t.Error("Edit(no fields) error = nil, want caller-contract error")
	} else if code, coded := apperr.CodeOf(err); coded {
		t.Errorf("Edit(no fields) error code = %q, want uncoded caller-contract error", code)
	}
	title := "missing"
	if _, err := tasks.Edit(ctx, 99, task.EditFields{Title: &title}, "2026-01-01T00:00:00.000Z"); errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(missing) error = %v, want not_found", err)
	}
}

func TestLifecycleTransitionsPreserveTaskAndEnforceState(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	dueOn := "2026-08-03"
	deferUntil := "2026-08-01"
	created, err := tasks.Add(
		ctx,
		task.AddFields{Title: "task", Note: "note", DueOn: &dueOn, DeferUntil: &deferUntil},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	created, err = tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(dated task) error = %v", err)
	}
	doneAt := "2026-01-02T03:04:05.678Z"
	done, err := tasks.Done(ctx, created.ID, doneAt)
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if done.Status != "done" || done.DoneAt == nil || *done.DoneAt != doneAt || done.CancelledAt != nil {
		t.Errorf("Done() = %#v, want generated done state at supplied timestamp", done)
	}
	assertPreservedTaskFields(t, done, created)
	if done.UpdatedAt != doneAt {
		t.Errorf("Done() updated_at = %q, want %q", done.UpdatedAt, doneAt)
	}

	if _, err := tasks.Cancel(ctx, created.ID, "2026-01-03T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Cancel(done) error = %v, want conflict", err)
	}
	unchanged, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(done) error = %v", err)
	}
	if unchanged.DoneAt == nil || *unchanged.DoneAt != doneAt || unchanged.UpdatedAt != doneAt {
		t.Errorf("conflicting Cancel() changed task: %#v", unchanged)
	}

	reopenedAt := "2026-01-04T03:04:05.678Z"
	reopened, err := tasks.Reopen(ctx, created.ID, reopenedAt)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if reopened.Status != "open" || reopened.DoneAt != nil || reopened.CancelledAt != nil || reopened.UpdatedAt != reopenedAt {
		t.Errorf("Reopen() = %#v, want cleared lifecycle timestamps", reopened)
	}
	assertPreservedTaskFields(t, reopened, created)
	if _, err := tasks.Reopen(ctx, created.ID, "2026-01-05T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Reopen(open) error = %v, want conflict", err)
	}

	cancelledAt := "2026-01-06T03:04:05.678Z"
	cancelled, err := tasks.Cancel(ctx, created.ID, cancelledAt)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status != "cancelled" || cancelled.CancelledAt == nil || *cancelled.CancelledAt != cancelledAt || cancelled.DoneAt != nil || cancelled.UpdatedAt != cancelledAt {
		t.Errorf("Cancel() = %#v, want generated cancelled state at supplied timestamp", cancelled)
	}
	assertPreservedTaskFields(t, cancelled, created)
	if _, err := tasks.Done(ctx, created.ID, "2026-01-07T00:00:00.000Z"); errorCode(err) != apperr.Conflict {
		t.Errorf("Done(cancelled) error = %v, want conflict", err)
	}
}

func TestTaskLifecycleTransitionsAreBlockedByResolvedProject(t *testing.T) {
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
		project.AddFields{Title: "resolved"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	completeCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &container.ID, Title: "complete candidate"},
	)
	cancelCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &container.ID, Title: "cancel candidate"},
	)
	reopenCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &container.ID, Title: "reopen candidate"},
	)
	deleteCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &container.ID, Title: "delete candidate"},
	)
	reopenCandidate, err = tasks.Done(ctx, reopenCandidate.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Done(reopen candidate) error = %v", err)
	}
	if _, err := projects.Resolve(
		ctx,
		container.ID,
		project.ExitDone,
		"2026-01-03T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(project) error = %v", err)
	}

	operations := []struct {
		name  string
		apply func() error
	}{
		{
			name: "done open task",
			apply: func() error {
				_, err := tasks.Done(ctx, completeCandidate.ID, "2026-01-04T00:00:00.000Z")
				return err
			},
		},
		{
			name: "cancel open task",
			apply: func() error {
				_, err := tasks.Cancel(ctx, cancelCandidate.ID, "2026-01-04T00:00:00.000Z")
				return err
			},
		},
		{
			name: "reopen done task",
			apply: func() error {
				_, err := tasks.Reopen(ctx, reopenCandidate.ID, "2026-01-04T00:00:00.000Z")
				return err
			},
		},
		{
			name: "state conflict still prioritizes project",
			apply: func() error {
				_, err := tasks.Done(ctx, reopenCandidate.ID, "2026-01-04T00:00:00.000Z")
				return err
			},
		},
	}
	for _, operation := range operations {
		err := operation.apply()
		var resolvedProjects *project.ResolvedProjectsError
		if errorCode(err) != apperr.Conflict ||
			!strings.Contains(err.Error(), fmt.Sprint(container.ID)) ||
			!errors.As(err, &resolvedProjects) ||
			!reflect.DeepEqual(resolvedProjects.IDs, []int64{container.ID}) {
			t.Errorf("%s error = %v, want conflict naming the resolved project with the typed marker", operation.name, err)
		}
	}

	persistedComplete, err := tasks.Find(ctx, completeCandidate.ID)
	if err != nil {
		t.Fatalf("Find(complete candidate) error = %v", err)
	}
	persistedCancel, err := tasks.Find(ctx, cancelCandidate.ID)
	if err != nil {
		t.Fatalf("Find(cancel candidate) error = %v", err)
	}
	persistedReopen, err := tasks.Find(ctx, reopenCandidate.ID)
	if err != nil {
		t.Fatalf("Find(reopen candidate) error = %v", err)
	}
	if persistedComplete.Status != "open" || persistedCancel.Status != "open" ||
		persistedReopen.Status != "done" || persistedReopen.UpdatedAt != reopenCandidate.UpdatedAt {
		t.Errorf(
			"tasks after blocked transitions = %#v, %#v, %#v; want original states",
			persistedComplete,
			persistedCancel,
			persistedReopen,
		)
	}
	deleted, err := tasks.Delete(ctx, deleteCandidate.ID)
	if err != nil {
		t.Fatalf("Delete(task in resolved project) error = %v", err)
	}
	if !reflect.DeepEqual(deleted, deleteCandidate) {
		t.Errorf("Delete(task in resolved project) = %#v, want snapshot %#v", deleted, deleteCandidate)
	}
}

func TestTaskArchivedAreaLifecycleGuardsAndDeleteAllowance(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	directArea := addStoredArea(t, areas, area.AddFields{Title: "direct"})
	inheritedArea := addStoredArea(t, areas, area.AddFields{Title: "inherited"})
	openProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &inheritedArea.ID, Title: "open project"},
	)
	resolvedProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &inheritedArea.ID, Title: "resolved project"},
	)
	doneCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{AreaID: &directArea.ID, Title: "done candidate"},
	)
	cancelCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &openProject.ID, Title: "cancel candidate"},
	)
	reopenCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &openProject.ID, Title: "reopen candidate"},
	)
	combinedCandidate := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &resolvedProject.ID, Title: "combined candidate"},
	)
	deleteInherited := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &openProject.ID, Title: "delete inherited"},
	)
	var err error
	reopenCandidate, err = tasks.Done(
		ctx,
		reopenCandidate.ID,
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Done(reopen candidate fixture) error = %v", err)
	}
	if _, err := projects.Resolve(
		ctx,
		resolvedProject.ID,
		project.ExitDone,
		"2026-01-03T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(project fixture) error = %v", err)
	}
	archiveStoredAreas(t, storage, directArea.ID, inheritedArea.ID)

	operations := []struct {
		name      string
		areaID    int64
		projectID int64
		apply     func() error
	}{
		{
			name:   "done direct",
			areaID: directArea.ID,
			apply: func() error {
				_, operationErr := tasks.Done(ctx, doneCandidate.ID, "2026-01-04T00:00:00.000Z")
				return operationErr
			},
		},
		{
			name:   "cancel inherited",
			areaID: inheritedArea.ID,
			apply: func() error {
				_, operationErr := tasks.Cancel(ctx, cancelCandidate.ID, "2026-01-04T00:00:00.000Z")
				return operationErr
			},
		},
		{
			name:   "reopen inherited",
			areaID: inheritedArea.ID,
			apply: func() error {
				_, operationErr := tasks.Reopen(ctx, reopenCandidate.ID, "2026-01-04T00:00:00.000Z")
				return operationErr
			},
		},
		{
			name:      "resolved project and archived inherited area",
			areaID:    inheritedArea.ID,
			projectID: resolvedProject.ID,
			apply: func() error {
				_, operationErr := tasks.Done(ctx, combinedCandidate.ID, "2026-01-04T00:00:00.000Z")
				return operationErr
			},
		},
	}
	for _, operation := range operations {
		err := operation.apply()
		if errorCode(err) != apperr.Conflict {
			t.Errorf("%s error = %v, want conflict", operation.name, err)
			continue
		}
		assertArchivedAreaIDs(t, err, []int64{operation.areaID})
		if !strings.Contains(err.Error(), fmt.Sprintf("area %d", operation.areaID)) {
			t.Errorf("%s error = %v, want area ID %d", operation.name, err, operation.areaID)
		}
		if operation.projectID != 0 &&
			!strings.Contains(err.Error(), fmt.Sprintf("project %d", operation.projectID)) {
			t.Errorf("%s error = %v, want project ID %d", operation.name, err, operation.projectID)
		}
	}

	deleted, err := tasks.Delete(ctx, deleteInherited.ID)
	if err != nil || !reflect.DeepEqual(deleted, deleteInherited) {
		t.Errorf("Delete(task under archived area) = %#v, %v; want %#v", deleted, err, deleteInherited)
	}
}

func TestLifecycleMissingTaskReturnsNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	tests := []struct {
		name  string
		apply func() error
	}{
		{name: "done", apply: func() error { _, err := tasks.Done(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "cancel", apply: func() error { _, err := tasks.Cancel(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "reopen", apply: func() error { _, err := tasks.Reopen(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "delete", apply: func() error { _, err := tasks.Delete(ctx, 99); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.apply(); errorCode(err) != apperr.NotFound {
				t.Errorf("operation error = %v, want not_found", err)
			}
		})
	}
}

func TestDeleteReturnsSnapshotWithoutCompactingPositions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })

	dueOn := "2026-08-03"
	first, err := tasks.Add(
		ctx,
		task.AddFields{Title: "first", Note: "first note", DueOn: &dueOn},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	second, err := tasks.Add(ctx, task.AddFields{Title: "second", Note: "second note"}, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	deleted, err := tasks.Delete(ctx, first.ID)
	if err != nil {
		t.Fatalf("Delete(first) error = %v", err)
	}
	if !reflect.DeepEqual(deleted, first) {
		t.Errorf("Delete(first) = %#v, want snapshot %#v", deleted, first)
	}
	if _, err := tasks.Find(ctx, first.ID); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(deleted) error = %v, want not_found", err)
	}
	remaining, err := tasks.Find(ctx, second.ID)
	if err != nil {
		t.Fatalf("Find(second) error = %v", err)
	}
	if remaining.Position != second.Position {
		t.Errorf("remaining position = %d, want unchanged %d", remaining.Position, second.Position)
	}
}

func TestTaskDeleteFailsWhenTriggerIgnoresTheRow(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tasks := NewTasks(storage)
	created := addStoredTask(t, tasks, task.AddFields{Title: "ignored delete"})
	trigger := fmt.Sprintf(`
CREATE TRIGGER ignore_task_delete
BEFORE DELETE ON tasks
WHEN OLD.id = %d
BEGIN
    SELECT RAISE(IGNORE);
END
`, created.ID)
	if _, err := storage.database.ExecContext(ctx, trigger); err != nil {
		t.Fatalf("create task delete trigger: %v", err)
	}

	if _, err := tasks.Delete(ctx, created.ID); err == nil {
		t.Fatal("Delete() error = nil, want ignored deletion rejected")
	}
	persisted, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() after ignored delete error = %v", err)
	}
	if !reflect.DeepEqual(persisted, created) {
		t.Errorf("task after ignored delete = %#v, want unchanged %#v", persisted, created)
	}
}

func TestTaskTransactionUsesAmbientStateAndRollsBack(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	container := addStoredProject(t, projects, project.AddFields{Title: "container"})

	var added task.Task
	err := tasks.WithinTransaction(ctx, func(transaction task.Transaction) error {
		var operationErr error
		added, operationErr = transaction.Add(
			ctx,
			task.AddFields{Title: "must roll back"},
			"2026-01-02T00:00:00.000Z",
		)
		if operationErr != nil {
			return operationErr
		}

		bound := transaction.(*tasksCore)
		var projectID int64
		if operationErr = bound.executor.QueryRowContext(ctx, `
UPDATE projects
SET done_at = ?, updated_at = ?
WHERE id = ?
RETURNING id
`, "2026-01-03T00:00:00.000Z", "2026-01-03T00:00:00.000Z", container.ID).Scan(&projectID); operationErr != nil {
			return operationErr
		}
		_, operationErr = transaction.Add(
			ctx,
			task.AddFields{ProjectID: &container.ID, Title: "blocked by ambient resolve"},
			"2026-01-04T00:00:00.000Z",
		)
		return operationErr
	})
	if errorCode(err) != apperr.Conflict {
		t.Fatalf("WithinTransaction() error = %v, want conflict", err)
	}
	if _, findErr := tasks.Find(ctx, added.ID); errorCode(findErr) != apperr.NotFound {
		t.Errorf("Find(rolled-back task) error = %v, want not_found", findErr)
	}
	persistedProject, findErr := projects.Find(ctx, container.ID)
	if findErr != nil {
		t.Fatalf("Find(project after rollback) error = %v", findErr)
	}
	if persistedProject.Status != "open" || persistedProject.UpdatedAt != container.UpdatedAt {
		t.Errorf("project after rollback = %#v, want original %#v", persistedProject, container)
	}
}

func TestConcurrentDoneAndCancelExactlyOneSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	tasks := NewTasks(storage)
	t.Cleanup(func() { _ = storage.Close() })
	created, err := tasks.Add(ctx, task.AddFields{Title: "race"}, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, doneErr := tasks.Done(ctx, created.ID, "2026-01-02T00:00:00.000Z")
		results <- doneErr
	}()
	go func() {
		<-start
		_, cancelErr := tasks.Cancel(ctx, created.ID, "2026-01-03T00:00:00.000Z")
		results <- cancelErr
	}()
	close(start)

	successes := 0
	conflicts := 0
	for range 2 {
		result := <-results
		switch errorCode(result) {
		case "":
			successes++
		case apperr.Conflict:
			conflicts++
		default:
			t.Errorf("race error = %v, want nil or conflict", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("race successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}
	resolved, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if (resolved.DoneAt == nil) == (resolved.CancelledAt == nil) {
		t.Errorf("resolved task = %#v, want exactly one lifecycle timestamp", resolved)
	}
}

func assertPreservedTaskFields(t *testing.T, got, original task.Task) {
	t.Helper()
	if got.ID != original.ID || got.Title != original.Title || got.Note != original.Note ||
		!reflect.DeepEqual(got.DeferUntil, original.DeferUntil) ||
		!reflect.DeepEqual(got.DueOn, original.DueOn) ||
		got.Position != original.Position || got.CreatedAt != original.CreatedAt {
		t.Errorf("task fields changed: got %#v, original %#v", got, original)
	}
}

func errorCode(err error) apperr.Code {
	code, _ := apperr.CodeOf(err)
	return code
}

func TestResolvePathPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		database string
		dataHome string
		home     string
		want     string
	}{
		{
			name:     "flag",
			flag:     "flag.db",
			database: "environment.db",
			dataHome: "/data",
			home:     "/home/user",
			want:     "flag.db",
		},
		{
			name:     "environment",
			database: "environment.db",
			dataHome: "/data",
			home:     "/home/user",
			want:     "environment.db",
		},
		{
			name:     "XDG data home",
			dataHome: "/data",
			home:     "/home/user",
			want:     filepath.Join("/data", "gsd", "gsd.db"),
		},
		{
			name: "home fallback",
			home: "/home/user",
			want: filepath.Join("/home/user", ".local", "share", "gsd", "gsd.db"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GSD_DB", test.database)
			t.Setenv("XDG_DATA_HOME", test.dataHome)
			t.Setenv("HOME", test.home)

			got, err := ResolvePath(test.flag)
			if err != nil {
				t.Fatalf("ResolvePath() error = %v", err)
			}
			if got != test.want {
				t.Errorf("ResolvePath() = %q, want %q", got, test.want)
			}
		})
	}
}
