package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
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
