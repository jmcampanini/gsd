package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
)

const taskColumns = `id, title, note, defer_until, due_on, done_at, cancelled_at, status, position, created_at, updated_at`

type Tasks struct {
	db *DB
}

func NewTasks(database *DB) *Tasks {
	return &Tasks{db: database}
}

func (s *Tasks) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	query := `
INSERT INTO tasks (title, note, defer_until, due_on, position, created_at, updated_at)
SELECT ?, ?, ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
FROM tasks
RETURNING ` + taskColumns

	row := s.db.database.QueryRowContext(
		ctx,
		query,
		fields.Title,
		fields.Note,
		fields.DeferUntil,
		fields.DueOn,
		timestamp,
		timestamp,
	)
	created, err := scanTask(row)
	if err != nil {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}

	return created, nil
}

func (s *Tasks) Inbox(ctx context.Context) ([]task.Task, error) {
	rows, err := s.db.database.QueryContext(ctx, "SELECT "+taskColumns+" FROM inbox ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}

	return collectTasks(rows, "scan inbox task", "iterate inbox")
}

func (s *Tasks) Available(ctx context.Context) ([]task.Task, error) {
	rows, err := s.db.database.QueryContext(ctx, "SELECT "+taskColumns+" FROM available ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query available tasks: %w", err)
	}

	return collectTasks(rows, "scan available task", "iterate available tasks")
}

func (s *Tasks) Find(ctx context.Context, id int64) (task.Task, error) {
	row := s.db.database.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = ?", id)
	found, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, apperr.New(apperr.NotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("find task: %w", err)
	}

	return found, nil
}

func (s *Tasks) List(ctx context.Context, options task.ListOptions) ([]task.Task, error) {
	conditions := make([]string, 0, 2)
	arguments := make([]any, 0, 1)
	switch options.Status {
	case task.ListStatusOpen, task.ListStatusDone, task.ListStatusCancelled:
		conditions = append(conditions, "status = ?")
		arguments = append(arguments, options.Status)
	case task.ListStatusAll:
	default:
		return nil, fmt.Errorf("invalid list status %q", options.Status)
	}

	switch options.Date {
	case task.DateSelectorNone:
	case task.DateSelectorDue:
		conditions = append(conditions, "due_on IS NOT NULL")
	case task.DateSelectorOverdue:
		conditions = append(
			conditions,
			"status = 'open' AND due_on < date('now', 'localtime')",
		)
	case task.DateSelectorDeferred:
		conditions = append(conditions, "defer_until > date('now', 'localtime')")
	default:
		return nil, fmt.Errorf("invalid date selector %q", options.Date)
	}

	query := "SELECT " + taskColumns + " FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY position, id"

	rows, err := s.db.database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	return collectTasks(rows, "scan listed task", "iterate listed tasks")
}

func (s *Tasks) Edit(
	ctx context.Context,
	id int64,
	fields task.EditFields,
	timestamp string,
) (task.Task, error) {
	assignments := make([]string, 0, 6)
	arguments := make([]any, 0, 7)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}
	if fields.DueOn.Set != nil {
		assignments = append(assignments, "due_on = ?")
		arguments = append(arguments, *fields.DueOn.Set)
	}
	if fields.DueOn.Clear {
		assignments = append(assignments, "due_on = NULL")
	}
	if fields.DeferUntil.Set != nil {
		assignments = append(assignments, "defer_until = ?")
		arguments = append(arguments, *fields.DeferUntil.Set)
	}
	if fields.DeferUntil.Clear {
		assignments = append(assignments, "defer_until = NULL")
	}
	if len(assignments) == 0 {
		return task.Task{}, errors.New("edit requires at least one field")
	}

	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, id)
	query := "UPDATE tasks SET " + strings.Join(assignments, ", ") + " WHERE id = ? RETURNING " + taskColumns
	edited, err := scanTask(s.db.database.QueryRowContext(ctx, query, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, apperr.New(apperr.NotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("edit task: %w", err)
	}

	return edited, nil
}

func (s *Tasks) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
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

func (s *Tasks) Cancel(ctx context.Context, id int64, timestamp string) (task.Task, error) {
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

func (s *Tasks) Reopen(ctx context.Context, id int64, timestamp string) (task.Task, error) {
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

func (s *Tasks) Delete(ctx context.Context, id int64) (task.Task, error) {
	row := s.db.database.QueryRowContext(ctx, "DELETE FROM tasks WHERE id = ? RETURNING "+taskColumns, id)
	deleted, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, apperr.New(apperr.NotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("delete task: %w", err)
	}

	return deleted, nil
}

func (s *Tasks) transition(
	ctx context.Context,
	id int64,
	action string,
	query string,
	arguments ...any,
) (task.Task, error) {
	updated, err := scanTask(s.db.database.QueryRowContext(ctx, query, arguments...))
	if err == nil {
		return updated, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("%s task: %w", action, err)
	}

	if _, findErr := s.Find(ctx, id); findErr != nil {
		return task.Task{}, findErr
	}

	return task.Task{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot %s task %d in its current state", action, id),
		err,
	)
}

type rowScanner interface {
	Scan(...any) error
}

func collectTasks(rows *sql.Rows, scanAction, iterateAction string) ([]task.Task, error) {
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]task.Task, 0)
	for rows.Next() {
		current, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", scanAction, err)
		}
		tasks = append(tasks, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", iterateAction, err)
	}

	return tasks, nil
}

func scanTask(scanner rowScanner) (task.Task, error) {
	var value task.Task
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.Note,
		&value.DeferUntil,
		&value.DueOn,
		&value.DoneAt,
		&value.CancelledAt,
		&value.Status,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
	)

	return value, err
}
