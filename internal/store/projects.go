package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const projectColumns = `id, area_id, title, note, done_at, cancelled_at, status, position, created_at, updated_at`

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
	if s.database != nil {
		var created project.Project
		err := s.WithinTransaction(ctx, func(transaction project.Store) error {
			var err error
			created, err = transaction.Add(ctx, fields, timestamp)
			return err
		})
		if err != nil {
			return project.Project{}, err
		}
		return created, nil
	}

	if fields.AreaID != nil {
		archivedAreaIDs, err := s.archivedAreaIDs(ctx, *fields.AreaID)
		if err != nil {
			return project.Project{}, err
		}
		if len(archivedAreaIDs) > 0 {
			message := fmt.Sprintf("cannot add project to area %d while it is archived", *fields.AreaID)
			return project.Project{}, archivedAreasConflict(message, archivedAreaIDs, nil)
		}
	}

	areaID := nullableID(fields.AreaID)
	created, err := scanProject(s.executor.QueryRowContext(ctx, `
INSERT INTO projects (area_id, title, note, position, created_at, updated_at)
VALUES (?, ?, ?,
        COALESCE((SELECT MAX(position) FROM projects WHERE area_id IS ?), -1) + 1,
        ?, ?)
RETURNING `+projectColumns,
		areaID,
		fields.Title,
		fields.Note,
		areaID,
		timestamp,
		timestamp,
	))
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
	conditions := make([]string, 0, 1)
	arguments := make([]any, 0, 1)
	switch options.Status {
	case project.ListStatusOpen, project.ListStatusDone, project.ListStatusCancelled:
		conditions = append(conditions, "status = ?")
		arguments = append(arguments, options.Status)
	case project.ListStatusAll:
	default:
		return nil, fmt.Errorf("invalid project list status %q", options.Status)
	}

	if options.AreaID != nil {
		if s.database != nil {
			var listed []project.Project
			err := s.WithinTransaction(ctx, func(transaction project.Store) error {
				var err error
				listed, err = transaction.List(ctx, options)
				return err
			})
			if err != nil {
				return nil, err
			}
			return listed, nil
		}

		return s.listArea(ctx, *options.AreaID, conditions, arguments)
	}

	query := "SELECT " + projectColumns + " FROM projects"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
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
	if fields.Area.Set != nil && fields.Area.Clear {
		return project.Project{}, errors.New("area cannot be set and cleared")
	}
	contentChanged := fields.Title != nil || fields.Note != nil
	membershipRequested := fields.Area.Set != nil || fields.Area.Clear
	if !contentChanged && !membershipRequested {
		return project.Project{}, errors.New("edit requires at least one field")
	}
	if s.database != nil {
		var edited project.Project
		err := s.WithinTransaction(ctx, func(transaction project.Store) error {
			var err error
			edited, err = transaction.Edit(ctx, id, fields, timestamp)
			return err
		})
		if err != nil {
			return project.Project{}, err
		}
		return edited, nil
	}

	current, err := s.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	destination := current.AreaID
	if fields.Area.Set != nil {
		destination = fields.Area.Set
	} else if fields.Area.Clear {
		destination = nil
	}
	movement := !sameProjectArea(current.AreaID, destination)
	if movement {
		areaIDs := make([]int64, 0, 2)
		if destination != nil {
			areaIDs = append(areaIDs, *destination)
		}
		if current.AreaID != nil {
			areaIDs = append(areaIDs, *current.AreaID)
		}
		if err := s.validateActiveAreas(ctx, fmt.Sprintf("move project %d", id), areaIDs...); err != nil {
			return project.Project{}, err
		}
	}

	assignments := make([]string, 0, 5)
	arguments := make([]any, 0, 6)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}
	if movement {
		areaID := nullableID(destination)
		assignments = append(
			assignments,
			`position = COALESCE((
    SELECT MAX(sibling.position)
    FROM projects AS sibling
    WHERE sibling.area_id IS ?
), -1) + 1`,
			"area_id = ?",
		)
		arguments = append(arguments, areaID, areaID)
	}
	if contentChanged || movement {
		assignments = append(assignments, "updated_at = ?")
		arguments = append(arguments, timestamp)
	} else {
		return current, nil
	}

	query := "UPDATE projects SET " + strings.Join(assignments, ", ") +
		" WHERE id = ? RETURNING " + projectColumns
	arguments = append(arguments, id)
	edited, err := scanProject(s.executor.QueryRowContext(ctx, query, arguments...))
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
	if s.database != nil {
		var resolved project.Project
		err := s.WithinTransaction(ctx, func(transaction project.Store) error {
			var err error
			resolved, err = transaction.Resolve(ctx, id, exit, timestamp)
			return err
		})
		if err != nil {
			return project.Project{}, err
		}
		return resolved, nil
	}

	current, err := s.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	if current.AreaID != nil {
		if err := s.validateActiveAreas(
			ctx,
			fmt.Sprintf("%s project %d", action, id),
			*current.AreaID,
		); err != nil {
			return project.Project{}, err
		}
	}
	if current.DoneAt != nil || current.CancelledAt != nil {
		return project.Project{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("cannot %s project %d in its current state", action, id),
			nil,
		)
	}

	query := "UPDATE projects SET " + column + " = ?, updated_at = ? WHERE id = ? RETURNING " + projectColumns
	resolved, err := scanProject(s.executor.QueryRowContext(ctx, query, timestamp, timestamp, id))
	if err != nil {
		return project.Project{}, fmt.Errorf("%s project: %w", action, err)
	}

	return resolved, nil
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
	if s.database != nil {
		var reopened project.Project
		err := s.WithinTransaction(ctx, func(transaction project.Store) error {
			var err error
			reopened, err = transaction.Reopen(ctx, id, timestamp)
			return err
		})
		if err != nil {
			return project.Project{}, err
		}
		return reopened, nil
	}

	current, err := s.Find(ctx, id)
	if err != nil {
		return project.Project{}, err
	}
	if current.AreaID != nil {
		if err := s.validateActiveAreas(
			ctx,
			fmt.Sprintf("reopen project %d", id),
			*current.AreaID,
		); err != nil {
			return project.Project{}, err
		}
	}
	if current.DoneAt == nil && current.CancelledAt == nil {
		return project.Project{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("cannot reopen project %d in its current state", id),
			nil,
		)
	}

	reopened, err := scanProject(s.executor.QueryRowContext(ctx, `
UPDATE projects
SET done_at = NULL, cancelled_at = NULL, updated_at = ?
WHERE id = ?
RETURNING `+projectColumns, timestamp, id))
	if err != nil {
		return project.Project{}, fmt.Errorf("reopen project: %w", err)
	}

	return reopened, nil
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
		fmt.Sprintf("cannot delete project %d while it contains tasks", id),
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

	return withinImmediateTransaction(ctx, s.database, "project", func(connection *sql.Conn) error {
		return apply(&Projects{executor: connection})
	})
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
	return collectProjectRows(rows, "scan listed project", "iterate listed projects")
}

func collectProjectRows(rows *sql.Rows, scanAction, iterateAction string) ([]project.Project, error) {
	defer func() {
		_ = rows.Close()
	}()

	projects := make([]project.Project, 0)
	for rows.Next() {
		current, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", scanAction, err)
		}
		projects = append(projects, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", iterateAction, err)
	}

	return projects, nil
}

func scanProject(scanner rowScanner) (project.Project, error) {
	var value project.Project
	err := scanner.Scan(
		&value.ID,
		&value.AreaID,
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

func (s *Projects) listArea(
	ctx context.Context,
	areaID int64,
	conditions []string,
	arguments []any,
) ([]project.Project, error) {
	if _, err := s.findArea(ctx, areaID); err != nil {
		return nil, err
	}

	query := "SELECT " + projectColumns + " FROM projects WHERE area_id = ?"
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY position, id"
	scopedArguments := make([]any, 0, len(arguments)+1)
	scopedArguments = append(scopedArguments, areaID)
	scopedArguments = append(scopedArguments, arguments...)
	rows, err := s.executor.QueryContext(ctx, query, scopedArguments...)
	if err != nil {
		return nil, fmt.Errorf("list area projects: %w", err)
	}

	return collectProjectRows(rows, "scan listed area project", "iterate listed area projects")
}

func (s *Projects) archivedAreaIDs(ctx context.Context, ids ...int64) ([]int64, error) {
	archived := make([]int64, 0, len(ids))
	for _, id := range ids {
		found, err := s.findArea(ctx, id)
		if err != nil {
			return nil, err
		}
		if found.ArchivedAt != nil {
			archived = append(archived, found.ID)
		}
	}

	return sortedUniqueIDs(archived), nil
}

func (s *Projects) validateActiveAreas(ctx context.Context, action string, ids ...int64) error {
	archived, err := s.archivedAreaIDs(ctx, ids...)
	if err != nil {
		return err
	}
	if len(archived) == 0 {
		return nil
	}

	return taskBlockersConflict(action, nil, archived, nil)
}

func (s *Projects) findArea(ctx context.Context, id int64) (area.Area, error) {
	return (&Areas{executor: s.executor}).Find(ctx, id)
}

func sameProjectArea(left *int64, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
