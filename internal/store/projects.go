package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const projectColumns = `id, title, note, done_at, cancelled_at, status, position, created_at, updated_at`

type Projects struct {
	database *DB
	executor projectExecutor
}

type projectExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewProjects(database *DB) *Projects {
	return &Projects{database: database, executor: database.database}
}

func (s *Projects) Add(
	ctx context.Context,
	fields project.AddFields,
	timestamp string,
) (project.Project, error) {
	row := s.executor.QueryRowContext(ctx, `
INSERT INTO projects (title, note, position, created_at, updated_at)
SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
FROM projects
RETURNING `+projectColumns, fields.Title, fields.Note, timestamp, timestamp)
	created, err := scanProject(row)
	if err != nil {
		return project.Project{}, fmt.Errorf("insert project: %w", err)
	}

	return created, nil
}

func (s *Projects) Find(ctx context.Context, id int64) (project.Project, error) {
	row := s.executor.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", id)
	found, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, apperr.New(apperr.NotFound, fmt.Sprintf("no project %d", id), err)
	}
	if err != nil {
		return project.Project{}, fmt.Errorf("find project: %w", err)
	}

	return found, nil
}

func (s *Projects) List(ctx context.Context, options project.ListOptions) ([]project.Project, error) {
	query := "SELECT " + projectColumns + " FROM projects"
	arguments := make([]any, 0, 1)
	switch options.Status {
	case project.ListStatusOpen, project.ListStatusDone, project.ListStatusCancelled:
		query += " WHERE status = ?"
		arguments = append(arguments, options.Status)
	case project.ListStatusAll:
	default:
		return nil, fmt.Errorf("invalid project list status %q", options.Status)
	}
	query += " ORDER BY position, id"

	rows, err := s.executor.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	return collectProjects(rows)
}

func (s *Projects) Edit(
	ctx context.Context,
	id int64,
	fields project.EditFields,
	timestamp string,
) (project.Project, error) {
	assignments := make([]string, 0, 3)
	arguments := make([]any, 0, 4)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}
	if len(assignments) == 0 {
		return project.Project{}, errors.New("edit requires at least one field")
	}

	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, id)
	query := "UPDATE projects SET " + strings.Join(assignments, ", ") +
		" WHERE id = ? RETURNING " + projectColumns
	edited, err := scanProject(s.executor.QueryRowContext(ctx, query, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, apperr.New(apperr.NotFound, fmt.Sprintf("no project %d", id), err)
	}
	if err != nil {
		return project.Project{}, fmt.Errorf("edit project: %w", err)
	}

	return edited, nil
}

func (s *Projects) Resolve(
	ctx context.Context,
	id int64,
	exit project.Exit,
	timestamp string,
) (project.Project, error) {
	var column string
	var action string
	switch exit {
	case project.ExitDone:
		column = "done_at"
		action = "complete"
	case project.ExitCancelled:
		column = "cancelled_at"
		action = "cancel"
	default:
		return project.Project{}, fmt.Errorf("invalid project exit %q", exit)
	}

	query := "UPDATE projects SET " + column + " = ?, updated_at = ? " +
		"WHERE id = ? AND done_at IS NULL AND cancelled_at IS NULL RETURNING " + projectColumns
	resolved, err := scanProject(s.executor.QueryRowContext(ctx, query, timestamp, timestamp, id))
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("%s project: %w", action, err)
	}
	if _, findErr := s.Find(ctx, id); findErr != nil {
		return project.Project{}, findErr
	}

	return project.Project{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot %s project %d in its current state", action, id),
		err,
	)
}

func (s *Projects) CancelOpenTasks(
	ctx context.Context,
	projectID int64,
	timestamp string,
) ([]task.Task, error) {
	rows, err := s.executor.QueryContext(ctx, `
UPDATE tasks
SET cancelled_at = ?, updated_at = ?
WHERE project_id = ? AND done_at IS NULL AND cancelled_at IS NULL
RETURNING `+taskColumns, timestamp, timestamp, projectID)
	if err != nil {
		return nil, fmt.Errorf("cancel open project tasks: %w", err)
	}

	cancelled, err := collectTasks(rows, "scan cancelled project task", "iterate cancelled project tasks")
	if err != nil {
		return nil, err
	}
	sortTasks(cancelled)

	return cancelled, nil
}

func (s *Projects) Reopen(
	ctx context.Context,
	id int64,
	timestamp string,
) (project.Project, error) {
	reopened, err := scanProject(s.executor.QueryRowContext(ctx, `
UPDATE projects
SET done_at = NULL, cancelled_at = NULL, updated_at = ?
WHERE id = ? AND (done_at IS NOT NULL OR cancelled_at IS NOT NULL)
RETURNING `+projectColumns, timestamp, id))
	if err == nil {
		return reopened, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("reopen project: %w", err)
	}
	if _, findErr := s.Find(ctx, id); findErr != nil {
		return project.Project{}, findErr
	}

	return project.Project{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot reopen project %d in its current state", id),
		err,
	)
}

func (s *Projects) Delete(ctx context.Context, id int64) (project.Project, error) {
	deleted, err := scanProject(s.executor.QueryRowContext(ctx, `
DELETE FROM projects
WHERE id = ?
  AND NOT EXISTS (SELECT 1 FROM tasks WHERE project_id = projects.id)
RETURNING `+projectColumns, id))
	if err == nil {
		return deleted, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("delete project: %w", err)
	}
	if _, findErr := s.Find(ctx, id); findErr != nil {
		return project.Project{}, findErr
	}

	return project.Project{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf(
			"cannot delete project %d while it contains tasks; use --recursive to delete the project and its tasks",
			id,
		),
		err,
	)
}

func (s *Projects) DeleteTasks(ctx context.Context, projectID int64) ([]task.Task, error) {
	rows, err := s.executor.QueryContext(
		ctx,
		"DELETE FROM tasks WHERE project_id = ? RETURNING "+taskColumns,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("delete project tasks: %w", err)
	}

	deleted, err := collectTasks(rows, "scan deleted project task", "iterate deleted project tasks")
	if err != nil {
		return nil, err
	}
	sortTasks(deleted)

	return deleted, nil
}

func (s *Projects) WithinTransaction(
	ctx context.Context,
	apply func(project.Store) error,
) error {
	if s.database == nil {
		return errors.New("nested project transactions are not supported")
	}

	connection, err := s.database.database.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve project transaction connection: %w", err)
	}
	defer func() {
		_ = connection.Close()
	}()

	if _, err := connection.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin project transaction: %w", err)
	}
	transactionOpen := true
	defer func() {
		if transactionOpen {
			_, _ = connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		}
	}()

	transaction := &Projects{executor: connection}
	if err := apply(transaction); err != nil {
		if _, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback project transaction: %w", rollbackErr))
		}
		transactionOpen = false
		return err
	}
	if _, err := connection.ExecContext(ctx, "COMMIT"); err != nil {
		if _, rollbackErr := connection.ExecContext(context.WithoutCancel(ctx), "ROLLBACK"); rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("commit project transaction: %w", err),
				fmt.Errorf("rollback project transaction: %w", rollbackErr),
			)
		}
		transactionOpen = false
		return fmt.Errorf("commit project transaction: %w", err)
	}
	transactionOpen = false

	return nil
}

func sortTasks(tasks []task.Task) {
	sort.Slice(tasks, func(left, right int) bool {
		if tasks[left].Position != tasks[right].Position {
			return tasks[left].Position < tasks[right].Position
		}

		return tasks[left].ID < tasks[right].ID
	})
}

func collectProjects(rows *sql.Rows) ([]project.Project, error) {
	defer func() {
		_ = rows.Close()
	}()

	projects := make([]project.Project, 0)
	for rows.Next() {
		current, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed project: %w", err)
		}
		projects = append(projects, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed projects: %w", err)
	}

	return projects, nil
}

func scanProject(scanner rowScanner) (project.Project, error) {
	var value project.Project
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
