package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/task"
)

func TestOpenBootstrapsReducedSchemaAndConfiguresConnections(t *testing.T) {
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
	if version != schemaRevision {
		t.Errorf("user_version = %d, want %d", version, schemaRevision)
	}

	var strict int
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT strict FROM pragma_table_list WHERE name = 'tasks'",
	).Scan(&strict); err != nil {
		t.Fatalf("read tasks table metadata: %v", err)
	}
	if strict != 1 {
		t.Errorf("tasks strict = %d, want 1", strict)
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

func TestTaskServiceUsesGeneratedStatusAndPositionsAcrossResolvedTasks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = storage.Close()
	})
	service := task.NewService(storage)

	first, err := service.Add(ctx, "first", "first note")
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	if first.Status != "open" || first.Position != 0 {
		t.Errorf("first task = %#v, want open at position 0", first)
	}
	if first.CreatedAt != first.UpdatedAt {
		t.Errorf("created_at = %q, updated_at = %q, want equal", first.CreatedAt, first.UpdatedAt)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", first.CreatedAt); err != nil {
		t.Errorf("created_at = %q, want UTC milliseconds: %v", first.CreatedAt, err)
	}

	resolvedAt := "2026-07-27T12:00:00.000Z"
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE tasks SET done_at = ?, updated_at = ? WHERE id = ?",
		resolvedAt,
		resolvedAt,
		first.ID,
	); err != nil {
		t.Fatalf("resolve first task: %v", err)
	}

	second, err := service.Add(ctx, "second", "")
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	if second.Position != 1 {
		t.Errorf("second position = %d, want 1", second.Position)
	}

	inbox, err := service.Inbox(ctx)
	if err != nil {
		t.Fatalf("Inbox() error = %v", err)
	}
	if len(inbox) != 1 || inbox[0].ID != second.ID {
		t.Errorf("Inbox() = %#v, want only second task", inbox)
	}

	shown, err := service.Show(ctx, first.ID)
	if err != nil {
		t.Fatalf("Show(first) error = %v", err)
	}
	if shown.Status != "done" || shown.DoneAt == nil || *shown.DoneAt != resolvedAt {
		t.Errorf("Show(first) = %#v, want generated done status", shown)
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
			code, ok := task.ErrorCodeOf(err)
			if !ok || code != task.ErrorConflict {
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
	t.Cleanup(func() {
		_ = storage.Close()
	})

	_, err = storage.Find(context.Background(), 99)
	if err == nil {
		t.Fatal("Find() error = nil, want not_found")
	}
	code, ok := task.ErrorCodeOf(err)
	if !ok || code != task.ErrorNotFound {
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
	t.Cleanup(func() { _ = storage.Close() })

	created := make([]task.Task, 4)
	for index, title := range []string{"first", "second", "third", "fourth"} {
		created[index], err = storage.Add(ctx, title, "", "2026-01-01T00:00:00.000Z")
		if err != nil {
			t.Fatalf("Add(%s) error = %v", title, err)
		}
	}
	if _, err := storage.Done(ctx, created[1].ID, "2026-01-02T00:00:00.000Z"); err != nil {
		t.Fatalf("Done(second) error = %v", err)
	}
	if _, err := storage.Cancel(ctx, created[2].ID, "2026-01-03T00:00:00.000Z"); err != nil {
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
		listed, listErr := storage.List(ctx, test.status)
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

func TestEditAtomicallyUpdatesRequestedFieldsAndReturnsTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	created, err := storage.Add(ctx, "original", "original note", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	done, err := storage.Done(ctx, created.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}

	title := "  revised  "
	titleEdited, err := storage.Edit(
		ctx,
		created.ID,
		task.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(title) error = %v", err)
	}
	if titleEdited.Title != title || titleEdited.Note != done.Note {
		t.Errorf("Edit(title) = %#v, want exact title and preserved note", titleEdited)
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

	note := ""
	noteEdited, err := storage.Edit(
		ctx,
		created.ID,
		task.EditFields{Note: &note},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(note) error = %v", err)
	}
	if noteEdited.Title != title || noteEdited.Note != "" {
		t.Errorf("Edit(note) = %#v, want preserved title and cleared note", noteEdited)
	}
	persisted, err := storage.Find(ctx, created.ID)
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
	t.Cleanup(func() { _ = storage.Close() })

	if _, err := storage.Edit(ctx, 1, task.EditFields{}, "2026-01-01T00:00:00.000Z"); errorCode(err) != task.ErrorInvalidArgument {
		t.Errorf("Edit(no fields) error = %v, want invalid_argument", err)
	}
	title := "missing"
	if _, err := storage.Edit(ctx, 99, task.EditFields{Title: &title}, "2026-01-01T00:00:00.000Z"); errorCode(err) != task.ErrorNotFound {
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
	t.Cleanup(func() { _ = storage.Close() })

	created, err := storage.Add(ctx, "task", "note", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	doneAt := "2026-01-02T03:04:05.678Z"
	done, err := storage.Done(ctx, created.ID, doneAt)
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

	if _, err := storage.Cancel(ctx, created.ID, "2026-01-03T00:00:00.000Z"); errorCode(err) != task.ErrorConflict {
		t.Errorf("Cancel(done) error = %v, want conflict", err)
	}
	unchanged, err := storage.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(done) error = %v", err)
	}
	if unchanged.DoneAt == nil || *unchanged.DoneAt != doneAt || unchanged.UpdatedAt != doneAt {
		t.Errorf("conflicting Cancel() changed task: %#v", unchanged)
	}

	reopenedAt := "2026-01-04T03:04:05.678Z"
	reopened, err := storage.Reopen(ctx, created.ID, reopenedAt)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if reopened.Status != "open" || reopened.DoneAt != nil || reopened.CancelledAt != nil || reopened.UpdatedAt != reopenedAt {
		t.Errorf("Reopen() = %#v, want cleared lifecycle timestamps", reopened)
	}
	assertPreservedTaskFields(t, reopened, created)
	if _, err := storage.Reopen(ctx, created.ID, "2026-01-05T00:00:00.000Z"); errorCode(err) != task.ErrorConflict {
		t.Errorf("Reopen(open) error = %v, want conflict", err)
	}

	cancelledAt := "2026-01-06T03:04:05.678Z"
	cancelled, err := storage.Cancel(ctx, created.ID, cancelledAt)
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelled.Status != "cancelled" || cancelled.CancelledAt == nil || *cancelled.CancelledAt != cancelledAt || cancelled.DoneAt != nil || cancelled.UpdatedAt != cancelledAt {
		t.Errorf("Cancel() = %#v, want generated cancelled state at supplied timestamp", cancelled)
	}
	assertPreservedTaskFields(t, cancelled, created)
	if _, err := storage.Done(ctx, created.ID, "2026-01-07T00:00:00.000Z"); errorCode(err) != task.ErrorConflict {
		t.Errorf("Done(cancelled) error = %v, want conflict", err)
	}
}

func TestLifecycleMissingTaskReturnsNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })

	tests := []struct {
		name  string
		apply func() error
	}{
		{name: "done", apply: func() error { _, err := storage.Done(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "cancel", apply: func() error { _, err := storage.Cancel(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "reopen", apply: func() error { _, err := storage.Reopen(ctx, 99, "2026-01-01T00:00:00.000Z"); return err }},
		{name: "delete", apply: func() error { _, err := storage.Delete(ctx, 99); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.apply(); errorCode(err) != task.ErrorNotFound {
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
	t.Cleanup(func() { _ = storage.Close() })

	first, err := storage.Add(ctx, "first", "first note", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(first) error = %v", err)
	}
	second, err := storage.Add(ctx, "second", "second note", "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(second) error = %v", err)
	}
	deleted, err := storage.Delete(ctx, first.ID)
	if err != nil {
		t.Fatalf("Delete(first) error = %v", err)
	}
	if !reflect.DeepEqual(deleted, first) {
		t.Errorf("Delete(first) = %#v, want snapshot %#v", deleted, first)
	}
	if _, err := storage.Find(ctx, first.ID); errorCode(err) != task.ErrorNotFound {
		t.Errorf("Find(deleted) error = %v, want not_found", err)
	}
	remaining, err := storage.Find(ctx, second.ID)
	if err != nil {
		t.Fatalf("Find(second) error = %v", err)
	}
	if remaining.Position != second.Position {
		t.Errorf("remaining position = %d, want unchanged %d", remaining.Position, second.Position)
	}
}

func TestConcurrentDoneAndCancelExactlyOneSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	created, err := storage.Add(ctx, "race", "", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, doneErr := storage.Done(ctx, created.ID, "2026-01-02T00:00:00.000Z")
		results <- doneErr
	}()
	go func() {
		<-start
		_, cancelErr := storage.Cancel(ctx, created.ID, "2026-01-03T00:00:00.000Z")
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
		case task.ErrorConflict:
			conflicts++
		default:
			t.Errorf("race error = %v, want nil or conflict", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Errorf("race successes/conflicts = %d/%d, want 1/1", successes, conflicts)
	}
	resolved, err := storage.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if (resolved.DoneAt == nil) == (resolved.CancelledAt == nil) {
		t.Errorf("resolved task = %#v, want exactly one lifecycle timestamp", resolved)
	}
}

func assertPreservedTaskFields(t *testing.T, got, original task.Task) {
	t.Helper()
	if got.ID != original.ID || got.Title != original.Title || got.Note != original.Note || got.Position != original.Position || got.CreatedAt != original.CreatedAt {
		t.Errorf("task fields changed: got %#v, original %#v", got, original)
	}
}

func errorCode(err error) task.ErrorCode {
	code, _ := task.ErrorCodeOf(err)
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
