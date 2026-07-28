package store

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/jmcampanini/gsd/internal/task"
	_ "modernc.org/sqlite"
)

const (
	schemaRevision = 9001
	busyTimeoutMS  = 5000
	taskColumns    = `id, title, note, done_at, cancelled_at, status, position, created_at, updated_at`
)

//go:embed schema.sql
var schema string

type Store struct {
	database *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	database, err := sql.Open("sqlite", dataSourceName(absolutePath))
	if err != nil {
		return nil, fmt.Errorf("configure database: %w", err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := bootstrap(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}

	return &Store{database: database}, nil
}

func (s *Store) Close() error {
	return s.database.Close()
}

func (s *Store) Add(ctx context.Context, title, note, timestamp string) (task.Task, error) {
	query := `
INSERT INTO tasks (title, note, position, created_at, updated_at)
SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
FROM tasks
RETURNING ` + taskColumns

	row := s.database.QueryRowContext(ctx, query, title, note, timestamp, timestamp)
	created, err := scanTask(row)
	if err != nil {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}

	return created, nil
}

func (s *Store) Inbox(ctx context.Context) ([]task.Task, error) {
	rows, err := s.database.QueryContext(ctx, "SELECT "+taskColumns+" FROM inbox ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]task.Task, 0)
	for rows.Next() {
		current, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan inbox task: %w", scanErr)
		}
		tasks = append(tasks, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inbox: %w", err)
	}

	return tasks, nil
}

func (s *Store) Find(ctx context.Context, id int64) (task.Task, error) {
	row := s.database.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = ?", id)
	found, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, task.NewError(task.ErrorNotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("find task: %w", err)
	}

	return found, nil
}

func (s *Store) List(ctx context.Context, status task.ListStatus) ([]task.Task, error) {
	query := "SELECT " + taskColumns + " FROM tasks"
	switch status {
	case task.ListStatusOpen, task.ListStatusDone, task.ListStatusCancelled:
		query += " WHERE status = ? ORDER BY position, id"
	case task.ListStatusAll:
		query += " ORDER BY position, id"
	default:
		return nil, task.NewError(task.ErrorInvalidArgument, fmt.Sprintf("invalid list status %q", status), nil)
	}

	var (
		rows *sql.Rows
		err  error
	)
	if status == task.ListStatusAll {
		rows, err = s.database.QueryContext(ctx, query)
	} else {
		rows, err = s.database.QueryContext(ctx, query, status)
	}
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]task.Task, 0)
	for rows.Next() {
		current, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan listed task: %w", scanErr)
		}
		tasks = append(tasks, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed tasks: %w", err)
	}

	return tasks, nil
}

func (s *Store) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.transition(
		ctx,
		id,
		"complete",
		`UPDATE tasks
SET done_at = ?, updated_at = ?
WHERE id = ? AND done_at IS NULL AND cancelled_at IS NULL
RETURNING `+taskColumns,
		timestamp,
		timestamp,
		id,
	)
}

func (s *Store) Cancel(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.transition(
		ctx,
		id,
		"cancel",
		`UPDATE tasks
SET cancelled_at = ?, updated_at = ?
WHERE id = ? AND done_at IS NULL AND cancelled_at IS NULL
RETURNING `+taskColumns,
		timestamp,
		timestamp,
		id,
	)
}

func (s *Store) Reopen(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.transition(
		ctx,
		id,
		"reopen",
		`UPDATE tasks
SET done_at = NULL, cancelled_at = NULL, updated_at = ?
WHERE id = ? AND (done_at IS NOT NULL OR cancelled_at IS NOT NULL)
RETURNING `+taskColumns,
		timestamp,
		id,
	)
}

func (s *Store) Delete(ctx context.Context, id int64) (task.Task, error) {
	row := s.database.QueryRowContext(ctx, "DELETE FROM tasks WHERE id = ? RETURNING "+taskColumns, id)
	deleted, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, task.NewError(task.ErrorNotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("delete task: %w", err)
	}

	return deleted, nil
}

func (s *Store) transition(
	ctx context.Context,
	id int64,
	action string,
	query string,
	arguments ...any,
) (task.Task, error) {
	updated, err := scanTask(s.database.QueryRowContext(ctx, query, arguments...))
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("%s task: %w", action, err)
	}

	if _, findErr := s.Find(ctx, id); findErr != nil {
		return task.Task{}, findErr
	}

	return task.Task{}, task.NewError(
		task.ErrorConflict,
		fmt.Sprintf("cannot %s task %d in its current state", action, id),
		err,
	)
}

func ResolvePath(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if environmentPath := os.Getenv("GSD_DB"); environmentPath != "" {
		return environmentPath, nil
	}
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		return filepath.Join(dataHome, "gsd", "gsd.db"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".local", "share", "gsd", "gsd.db"), nil
}

func dataSourceName(path string) string {
	location := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := location.Query()
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", busyTimeoutMS))
	location.RawQuery = query.Encode()

	return location.String()
}

func bootstrap(ctx context.Context, database *sql.DB) error {
	version, err := userVersion(ctx, database)
	if err != nil {
		return err
	}
	if version == schemaRevision {
		return nil
	}
	if version != 0 {
		return revisionConflict(version)
	}

	connection, err := database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve bootstrap connection: %w", err)
	}
	defer func() {
		_ = connection.Close()
	}()

	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin database bootstrap: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	version, err = userVersion(ctx, connection)
	if err != nil {
		return err
	}
	if version == schemaRevision {
		if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
			return fmt.Errorf("commit database inspection: %w", err)
		}
		committed = true
		return nil
	}
	if version != 0 {
		return revisionConflict(version)
	}

	var objectCount int
	if err := connection.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM sqlite_schema
WHERE name NOT LIKE 'sqlite\_%' ESCAPE '\'
`).Scan(&objectCount); err != nil {
		return fmt.Errorf("inspect database schema: %w", err)
	}
	if objectCount != 0 {
		return task.NewError(
			task.ErrorConflict,
			"database is not empty; delete your development database and try again",
			nil,
		)
	}

	if _, err := connection.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create database schema: %w", err)
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit database bootstrap: %w", err)
	}
	committed = true

	return nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTask(scanner rowScanner) (task.Task, error) {
	var value task.Task
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.Note,
		&value.DoneAt,
		&value.CancelledAt,
		&value.Status,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
	)

	return value, err
}

func userVersion(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (int, error) {
	var version int
	if err := queryer.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read database revision: %w", err)
	}

	return version, nil
}

func revisionConflict(version int) error {
	return task.NewError(
		task.ErrorConflict,
		fmt.Sprintf(
			"database revision %d does not match required revision %d; delete your development database and try again",
			version,
			schemaRevision,
		),
		nil,
	)
}
