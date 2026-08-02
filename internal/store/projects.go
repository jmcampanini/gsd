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
	areaID := nullableID(fields.AreaID)
	created, err := scanProject(s.executor.QueryRowContext(ctx, `
INSERT INTO projects (area_id, title, note, position, created_at, updated_at)
SELECT ?, ?, ?,
       COALESCE((SELECT MAX(position) FROM projects WHERE area_id IS ?), -1) + 1,
       ?, ?
WHERE ? IS NULL OR EXISTS (
    SELECT 1 FROM areas WHERE id = ? AND archived_at IS NULL
)
RETURNING `+projectColumns,
		areaID,
		fields.Title,
		fields.Note,
		areaID,
		timestamp,
		timestamp,
		areaID,
		areaID,
	))
	if err == nil {
		return created, nil
	}
	if errors.Is(err, sql.ErrNoRows) && fields.AreaID != nil {
		found, findErr := s.findArea(ctx, *fields.AreaID)
		if findErr != nil {
			return project.Project{}, findErr
		}
		if found.ArchivedAt != nil {
			message := fmt.Sprintf("cannot add project to area %d while it is archived", *fields.AreaID)
			return project.Project{}, archivedAreasConflict(message, []int64{*fields.AreaID}, err)
		}
		return project.Project{}, fmt.Errorf("classify active area %d: %w", *fields.AreaID, err)
	}

	return project.Project{}, fmt.Errorf("insert project: %w", err)
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
		query := "WITH listed_projects AS (SELECT " + projectColumns +
			" FROM projects WHERE area_id = ?"
		if len(conditions) > 0 {
			query += " AND " + strings.Join(conditions, " AND ")
		}
		query += `
), requested_area AS (
    SELECT 1 FROM areas WHERE id = ?
)
SELECT * FROM listed_projects
UNION ALL
SELECT 0, NULL, '', '', NULL, NULL, '', 0, '', ''
FROM requested_area
WHERE NOT EXISTS (SELECT 1 FROM listed_projects)
ORDER BY position, id`
		scopedArguments := make([]any, 0, len(arguments)+2)
		scopedArguments = append(scopedArguments, *options.AreaID)
		scopedArguments = append(scopedArguments, arguments...)
		scopedArguments = append(scopedArguments, *options.AreaID)

		rows, err := s.executor.QueryContext(ctx, query, scopedArguments...)
		if err != nil {
			return nil, fmt.Errorf("list area projects: %w", err)
		}

		return collectAreaProjects(rows, *options.AreaID)
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

	assignments := make([]string, 0, 5)
	arguments := make([]any, 0, 10)
	if fields.Title != nil {
		assignments = append(assignments, "title = ?")
		arguments = append(arguments, *fields.Title)
	}
	if fields.Note != nil {
		assignments = append(assignments, "note = ?")
		arguments = append(arguments, *fields.Note)
	}

	contentChanged := len(assignments) > 0
	membershipRequested := fields.Area.Set != nil || fields.Area.Clear
	if !contentChanged && !membershipRequested {
		return project.Project{}, errors.New("edit requires at least one field")
	}

	var destination any
	if fields.Area.Set != nil {
		destination = *fields.Area.Set
	}
	if membershipRequested {
		assignments = append(
			assignments,
			`position = CASE
    WHEN projects.area_id IS ? THEN projects.position
    ELSE COALESCE((
        SELECT MAX(sibling.position)
        FROM projects AS sibling
        WHERE sibling.area_id IS ?
    ), -1) + 1
END`,
			"area_id = ?",
		)
		arguments = append(arguments, destination, destination, destination)
	}

	if contentChanged {
		assignments = append(assignments, "updated_at = ?")
		arguments = append(arguments, timestamp)
	} else {
		assignments = append(
			assignments,
			"updated_at = CASE WHEN projects.area_id IS ? THEN projects.updated_at ELSE ? END",
		)
		arguments = append(arguments, destination, timestamp)
	}

	query := "UPDATE projects SET " + strings.Join(assignments, ", ") + " WHERE id = ?"
	arguments = append(arguments, id)
	if membershipRequested {
		query += ` AND (
    projects.area_id IS ?
    OR (
        (
            projects.area_id IS NULL
            OR EXISTS (
                SELECT 1 FROM areas AS source_area
                WHERE source_area.id = projects.area_id
                  AND source_area.archived_at IS NULL
            )
        )
        AND (
            ? IS NULL
            OR EXISTS (
                SELECT 1 FROM areas AS destination_area
                WHERE destination_area.id = ?
                  AND destination_area.archived_at IS NULL
            )
        )
    )
)`
		arguments = append(arguments, destination, destination, destination)
	}
	query += " RETURNING " + projectColumns

	edited, err := scanProject(s.executor.QueryRowContext(ctx, query, arguments...))
	if err == nil {
		return edited, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("edit project: %w", err)
	}
	current, findErr := s.Find(ctx, id)
	if findErr != nil {
		return project.Project{}, findErr
	}

	archivedAreaIDs := make([]int64, 0, 2)
	if fields.Area.Set != nil {
		destinationArea, destinationErr := s.findArea(ctx, *fields.Area.Set)
		if destinationErr != nil {
			return project.Project{}, destinationErr
		}
		if destinationArea.ArchivedAt != nil {
			archivedAreaIDs = append(archivedAreaIDs, destinationArea.ID)
		}
	}
	if current.AreaID != nil {
		sourceArea, sourceErr := s.findArea(ctx, *current.AreaID)
		if sourceErr != nil {
			return project.Project{}, sourceErr
		}
		if sourceArea.ArchivedAt != nil {
			archivedAreaIDs = append(archivedAreaIDs, sourceArea.ID)
		}
	}
	if len(archivedAreaIDs) > 0 {
		archivedAreaIDs = sortedUniqueIDs(archivedAreaIDs)
		message := fmt.Sprintf(
			"cannot move project %d while areas %s are archived",
			id,
			formatIDs(archivedAreaIDs),
		)
		if len(archivedAreaIDs) == 1 {
			message = fmt.Sprintf(
				"cannot move project %d while area %d is archived",
				id,
				archivedAreaIDs[0],
			)
		}
		return project.Project{}, archivedAreasConflict(message, archivedAreaIDs, err)
	}

	return project.Project{}, fmt.Errorf("edit project: %w", err)
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
		`WHERE id = ?
  AND done_at IS NULL
  AND cancelled_at IS NULL
  AND (
      area_id IS NULL
      OR EXISTS (
          SELECT 1 FROM areas
          WHERE areas.id = projects.area_id AND areas.archived_at IS NULL
      )
  )
RETURNING ` + projectColumns
	resolved, err := scanProject(s.executor.QueryRowContext(ctx, query, timestamp, timestamp, id))
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("%s project: %w", action, err)
	}
	current, findErr := s.Find(ctx, id)
	if findErr != nil {
		return project.Project{}, findErr
	}
	if current.AreaID != nil {
		governingArea, areaErr := s.findArea(ctx, *current.AreaID)
		if areaErr != nil {
			return project.Project{}, areaErr
		}
		if governingArea.ArchivedAt != nil {
			return project.Project{}, archivedAreasConflict(
				fmt.Sprintf(
					"cannot %s project %d while area %d is archived",
					action,
					id,
					governingArea.ID,
				),
				[]int64{governingArea.ID},
				err,
			)
		}
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
WHERE id = ?
  AND (done_at IS NOT NULL OR cancelled_at IS NOT NULL)
  AND (
      area_id IS NULL
      OR EXISTS (
          SELECT 1 FROM areas
          WHERE areas.id = projects.area_id AND areas.archived_at IS NULL
      )
  )
RETURNING `+projectColumns, timestamp, id))
	if err == nil {
		return reopened, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, fmt.Errorf("reopen project: %w", err)
	}
	current, findErr := s.Find(ctx, id)
	if findErr != nil {
		return project.Project{}, findErr
	}
	if current.AreaID != nil {
		governingArea, areaErr := s.findArea(ctx, *current.AreaID)
		if areaErr != nil {
			return project.Project{}, areaErr
		}
		if governingArea.ArchivedAt != nil {
			return project.Project{}, archivedAreasConflict(
				fmt.Sprintf("cannot reopen project %d while area %d is archived", id, governingArea.ID),
				[]int64{governingArea.ID},
				err,
			)
		}
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

func collectAreaProjects(rows *sql.Rows, areaID int64) ([]project.Project, error) {
	defer func() {
		_ = rows.Close()
	}()

	areaExists := false
	projects := make([]project.Project, 0)
	for rows.Next() {
		current, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed area project: %w", err)
		}
		areaExists = true
		if current.ID != 0 {
			projects = append(projects, current)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed area projects: %w", err)
	}
	if !areaExists {
		return nil, missingAreaError(areaID, sql.ErrNoRows)
	}

	return projects, nil
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

func (s *Projects) findArea(ctx context.Context, id int64) (area.Area, error) {
	return (&Areas{executor: s.executor}).Find(ctx, id)
}

func missingAreaError(id int64, cause error) error {
	return apperr.New(apperr.NotFound, fmt.Sprintf("no area %d", id), cause)
}
