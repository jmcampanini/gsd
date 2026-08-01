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

const taskColumns = `id, project_id, title, note, defer_until, due_on, done_at, cancelled_at, status, position, created_at, updated_at`

type Tasks struct {
	db *DB
}

func NewTasks(database *DB) *Tasks {
	return &Tasks{db: database}
}

func (s *Tasks) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	projectID := nullableID(fields.ProjectID)
	query := `
INSERT INTO tasks (
    project_id, title, note, defer_until, due_on, position, created_at, updated_at
)
SELECT ?, ?, ?, ?, ?,
       COALESCE((SELECT MAX(position) FROM tasks WHERE project_id IS ?), -1) + 1,
       ?, ?
WHERE ? IS NULL
   OR EXISTS (
       SELECT 1 FROM projects WHERE id = ? AND status = 'open'
   )
RETURNING ` + taskColumns

	row := s.db.database.QueryRowContext(
		ctx,
		query,
		projectID,
		fields.Title,
		fields.Note,
		fields.DeferUntil,
		fields.DueOn,
		projectID,
		timestamp,
		timestamp,
		projectID,
		projectID,
	)
	created, err := scanTask(row)
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) || fields.ProjectID == nil {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}
	if classification := s.classifyOpenProject(
		ctx,
		*fields.ProjectID,
		fmt.Sprintf("add task to project %d", *fields.ProjectID),
		err,
	); classification != nil {
		return task.Task{}, classification
	}

	return task.Task{}, fmt.Errorf("insert task: %w", err)
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
	if options.ProjectID != nil {
		if _, err := NewProjects(s.db).Find(ctx, *options.ProjectID); err != nil {
			return nil, err
		}
	}

	conditions := make([]string, 0, 3)
	arguments := make([]any, 0, 2)
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

	if options.ProjectID != nil {
		conditions = append(conditions, "project_id IS ?")
		arguments = append(arguments, *options.ProjectID)
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
	if fields.Project.Set != nil && fields.Project.Clear {
		return task.Task{}, errors.New("project cannot be set and cleared")
	}

	assignments := make([]string, 0, 9)
	arguments := make([]any, 0, 16)
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

	contentChanged := len(assignments) > 0
	membershipRequested := fields.Project.Set != nil || fields.Project.Clear
	if !contentChanged && !membershipRequested {
		return task.Task{}, errors.New("edit requires at least one field")
	}

	var destination any
	if fields.Project.Set != nil {
		destination = *fields.Project.Set
	}
	if membershipRequested {
		assignments = append(
			assignments,
			`position = CASE
    WHEN tasks.project_id IS ? THEN tasks.position
    ELSE COALESCE((
        SELECT MAX(sibling.position)
        FROM tasks AS sibling
        WHERE sibling.project_id IS ?
    ), -1) + 1
END`,
			"project_id = ?",
		)
		arguments = append(arguments, destination, destination, destination)
	}

	if contentChanged {
		assignments = append(assignments, "updated_at = ?")
		arguments = append(arguments, timestamp)
	} else {
		assignments = append(
			assignments,
			"updated_at = CASE WHEN tasks.project_id IS ? THEN tasks.updated_at ELSE ? END",
		)
		arguments = append(arguments, destination, timestamp)
	}

	query := "UPDATE tasks SET " + strings.Join(assignments, ", ") + " WHERE id = ?"
	arguments = append(arguments, id)
	if membershipRequested {
		query += ` AND (
    tasks.project_id IS ?
    OR (
        (
            tasks.project_id IS NULL
            OR EXISTS (
                SELECT 1
                FROM projects AS source_project
                WHERE source_project.id = tasks.project_id
                  AND source_project.status = 'open'
            )
        )
        AND (
            ? IS NULL
            OR EXISTS (
                SELECT 1
                FROM projects AS destination_project
                WHERE destination_project.id = ?
                  AND destination_project.status = 'open'
            )
        )
    )
)`
		arguments = append(arguments, destination, destination, destination)
	}
	query += " RETURNING " + taskColumns

	edited, err := scanTask(s.db.database.QueryRowContext(ctx, query, arguments...))
	if err == nil {
		return edited, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("edit task: %w", err)
	}
	if !membershipRequested {
		if _, findErr := s.Find(ctx, id); findErr != nil {
			return task.Task{}, findErr
		}
		return task.Task{}, fmt.Errorf("edit task: %w", err)
	}

	return task.Task{}, s.classifyMembershipEdit(ctx, id, fields.Project.Set, err)
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

func (s *Tasks) classifyMembershipEdit(
	ctx context.Context,
	taskID int64,
	destination *int64,
	cause error,
) error {
	current, err := s.Find(ctx, taskID)
	if err != nil {
		return err
	}
	if sameNullableID(current.ProjectID, destination) {
		return fmt.Errorf("edit task: %w", cause)
	}

	if destination != nil {
		if err := s.classifyOpenProject(
			ctx,
			*destination,
			fmt.Sprintf("move task %d into project %d", taskID, *destination),
			cause,
		); err != nil {
			return err
		}
	}
	if current.ProjectID != nil {
		if err := s.classifyOpenProject(
			ctx,
			*current.ProjectID,
			fmt.Sprintf("move task %d out of project %d", taskID, *current.ProjectID),
			cause,
		); err != nil {
			return err
		}
	}

	return fmt.Errorf("edit task: %w", cause)
}

func (s *Tasks) classifyOpenProject(
	ctx context.Context,
	projectID int64,
	action string,
	cause error,
) error {
	found, err := NewProjects(s.db).Find(ctx, projectID)
	if err != nil {
		return err
	}
	if found.Status != "open" {
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf("cannot %s while the project is resolved; reopen the project first", action),
			cause,
		)
	}

	return nil
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
		&value.ProjectID,
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

func nullableID(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}

func sameNullableID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
