package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const taskColumns = `id, project_id, area_id, title, note, defer_until, due_on, done_at, cancelled_at, status, position, created_at, updated_at`

const taskViewColumns = taskColumns + `, project_title, governing_area_id, governing_area_title`

type Tasks struct {
	db *DB
}

type taskContainerKind uint8

const (
	taskContainerInbox taskContainerKind = iota
	taskContainerProject
	taskContainerArea
)

type taskContainer struct {
	kind taskContainerKind
	id   int64
}

type taskDestinationExpression struct {
	selectSQL string
	arguments []any
}

func NewTasks(database *DB) *Tasks {
	return &Tasks{db: database}
}

func (s *Tasks) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	if fields.ProjectID != nil && fields.AreaID != nil {
		return task.Task{}, errors.New("task cannot have both project and area")
	}

	projectID := nullableID(fields.ProjectID)
	areaID := nullableID(fields.AreaID)
	query := `
INSERT INTO tasks (
    project_id, area_id, title, note, defer_until, due_on, position, created_at, updated_at
)
SELECT ?, ?, ?, ?, ?, ?,
       (
           SELECT COALESCE(MAX(position), -1) + 1
           FROM tasks
           WHERE project_id IS ? AND area_id IS ?
       ),
       ?, ?
WHERE (
        ? IS NULL
        OR EXISTS (
            SELECT 1 FROM projects WHERE id = ? AND status = 'open'
        )
      )
  AND (? IS NULL OR EXISTS (SELECT 1 FROM areas WHERE id = ?))
RETURNING ` + taskColumns

	row := s.db.database.QueryRowContext(
		ctx,
		query,
		projectID,
		areaID,
		fields.Title,
		fields.Note,
		fields.DeferUntil,
		fields.DueOn,
		projectID,
		areaID,
		timestamp,
		timestamp,
		projectID,
		projectID,
		areaID,
		areaID,
	)
	created, err := scanTask(row)
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}
	if fields.AreaID != nil {
		if _, findErr := NewAreas(s.db).Find(ctx, *fields.AreaID); findErr != nil {
			return task.Task{}, findErr
		}
	}
	if fields.ProjectID != nil {
		if classification := s.classifyOpenProject(
			ctx,
			*fields.ProjectID,
			fmt.Sprintf("add task to project %d", *fields.ProjectID),
			err,
		); classification != nil {
			return task.Task{}, classification
		}
	}

	return task.Task{}, fmt.Errorf("insert task: %w", err)
}

func (s *Tasks) Inbox(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.db.database.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM inbox ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}

	return collectViewTasks(rows, "scan inbox task", "iterate inbox")
}

func (s *Tasks) Available(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.db.database.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM available ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query available tasks: %w", err)
	}

	return collectViewTasks(rows, "scan available task", "iterate available tasks")
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
	if options.ProjectID != nil && options.AreaID != nil {
		return nil, errors.New("task list cannot filter by both project and area")
	}

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

	if options.ProjectID != nil {
		return s.listContained(
			ctx,
			"project_id",
			"projects",
			"project",
			*options.ProjectID,
			conditions,
			arguments,
		)
	}
	if options.AreaID != nil {
		return s.listContained(
			ctx,
			"area_id",
			"areas",
			"area",
			*options.AreaID,
			conditions,
			arguments,
		)
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

func (s *Tasks) listContained(
	ctx context.Context,
	column string,
	table string,
	noun string,
	containerID int64,
	conditions []string,
	arguments []any,
) ([]task.Task, error) {
	query := "WITH listed_tasks AS (SELECT " + taskColumns +
		" FROM tasks WHERE " + column + " = ?"
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += `
), requested_container AS (
    SELECT 1 FROM ` + table + ` WHERE id = ?
)
SELECT * FROM listed_tasks
UNION ALL
SELECT 0, NULL, NULL, '', '', NULL, NULL, NULL, NULL, '', 0, '', ''
FROM requested_container
WHERE NOT EXISTS (SELECT 1 FROM listed_tasks)
ORDER BY position, id`

	scopedArguments := make([]any, 0, len(arguments)+2)
	scopedArguments = append(scopedArguments, containerID)
	scopedArguments = append(scopedArguments, arguments...)
	scopedArguments = append(scopedArguments, containerID)

	rows, err := s.db.database.QueryContext(ctx, query, scopedArguments...)
	if err != nil {
		return nil, fmt.Errorf("list %s tasks: %w", noun, err)
	}

	return collectContainedTasks(rows, noun, containerID)
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
	if fields.Area.Set != nil && fields.Area.Clear {
		return task.Task{}, errors.New("area cannot be set and cleared")
	}
	if fields.Project.Set != nil && fields.Area.Set != nil {
		return task.Task{}, errors.New("task cannot have both project and area")
	}

	assignments := make([]string, 0, 10)
	arguments := make([]any, 0, 5)
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
	membershipRequested := fields.Project.Set != nil || fields.Project.Clear ||
		fields.Area.Set != nil || fields.Area.Clear
	if !contentChanged && !membershipRequested {
		return task.Task{}, errors.New("edit requires at least one field")
	}

	const (
		destinationProject = "(SELECT project_id FROM task_destination)"
		destinationArea    = "(SELECT area_id FROM task_destination)"
	)

	var destination taskDestinationExpression
	if membershipRequested {
		destination = taskDestinationFor(fields)
		assignments = append(
			assignments,
			`position = CASE
    WHEN tasks.project_id IS `+destinationProject+` AND tasks.area_id IS `+destinationArea+` THEN tasks.position
    ELSE COALESCE((
        SELECT MAX(sibling.position)
        FROM tasks AS sibling
        WHERE sibling.project_id IS `+destinationProject+`
          AND sibling.area_id IS `+destinationArea+`
    ), -1) + 1
END`,
			"project_id = "+destinationProject,
			"area_id = "+destinationArea,
		)
	}

	if contentChanged {
		assignments = append(assignments, "updated_at = ?")
		arguments = append(arguments, timestamp)
	} else {
		assignments = append(
			assignments,
			"updated_at = CASE WHEN tasks.project_id IS "+destinationProject+
				" AND tasks.area_id IS "+destinationArea+" THEN tasks.updated_at ELSE ? END",
		)
		arguments = append(arguments, timestamp)
	}

	query := ""
	if membershipRequested {
		query = `WITH task_destination(project_id, area_id) AS MATERIALIZED (
    SELECT ` + destination.selectSQL + `
    FROM tasks
    WHERE id = ?
)
`
		queryArguments := make([]any, 0, len(destination.arguments)+len(arguments)+2)
		queryArguments = append(queryArguments, destination.arguments...)
		queryArguments = append(queryArguments, id)
		arguments = append(queryArguments, arguments...)
	}
	query += "UPDATE tasks SET " + strings.Join(assignments, ", ") + " WHERE id = ?"
	arguments = append(arguments, id)
	if membershipRequested {
		query += ` AND (
    (tasks.project_id IS ` + destinationProject + ` AND tasks.area_id IS ` + destinationArea + `)
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
            ` + destinationProject + ` IS NULL
            OR EXISTS (
                SELECT 1
                FROM projects AS destination_project
                WHERE destination_project.id = ` + destinationProject + `
                  AND destination_project.status = 'open'
            )
        )
        AND (
            ` + destinationArea + ` IS NULL
            OR EXISTS (
                SELECT 1 FROM areas AS destination_area
                WHERE destination_area.id = ` + destinationArea + `
            )
        )
    )
)`
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

	return task.Task{}, s.classifyMembershipEdit(ctx, id, fields, err)
}

func taskDestinationFor(fields task.EditFields) taskDestinationExpression {
	switch {
	case fields.Project.Set != nil:
		return taskDestinationExpression{
			selectSQL: "?, NULL",
			arguments: []any{*fields.Project.Set},
		}
	case fields.Area.Set != nil:
		return taskDestinationExpression{
			selectSQL: "NULL, ?",
			arguments: []any{*fields.Area.Set},
		}
	case fields.Project.Clear && fields.Area.Clear:
		return taskDestinationExpression{selectSQL: "NULL, NULL"}
	case fields.Project.Clear:
		return taskDestinationExpression{selectSQL: "NULL, area_id"}
	case fields.Area.Clear:
		return taskDestinationExpression{selectSQL: "project_id, NULL"}
	default:
		panic("task destination requested without membership change")
	}
}

func (s *Tasks) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.transition(
		ctx,
		id,
		"complete",
		`UPDATE tasks
SET done_at = ?, updated_at = ?
WHERE id = ?
  AND done_at IS NULL
  AND cancelled_at IS NULL
  AND (
      project_id IS NULL
      OR EXISTS (
          SELECT 1 FROM projects
          WHERE projects.id = tasks.project_id AND projects.status = 'open'
      )
  )
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
WHERE id = ?
  AND done_at IS NULL
  AND cancelled_at IS NULL
  AND (
      project_id IS NULL
      OR EXISTS (
          SELECT 1 FROM projects
          WHERE projects.id = tasks.project_id AND projects.status = 'open'
      )
  )
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
WHERE id = ?
  AND (done_at IS NOT NULL OR cancelled_at IS NOT NULL)
  AND (
      project_id IS NULL
      OR EXISTS (
          SELECT 1 FROM projects
          WHERE projects.id = tasks.project_id AND projects.status = 'open'
      )
  )
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

	current, findErr := s.Find(ctx, id)
	if findErr != nil {
		return task.Task{}, findErr
	}
	if current.ProjectID != nil {
		container, projectErr := NewProjects(s.db).Find(ctx, *current.ProjectID)
		if projectErr != nil {
			return task.Task{}, projectErr
		}
		if container.Status != "open" {
			return task.Task{}, apperr.New(
				apperr.Conflict,
				fmt.Sprintf(
					"cannot %s task %d while project %d is resolved; reopen project %d first",
					action,
					id,
					container.ID,
					container.ID,
				),
				err,
			)
		}
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
	fields task.EditFields,
	cause error,
) error {
	current, err := s.Find(ctx, taskID)
	if err != nil {
		return err
	}

	source := taskContainerOf(current)
	destination := taskContainerAfterChange(source, fields)
	if source == destination {
		return fmt.Errorf("edit task: %w", cause)
	}

	var destinationProject *project.Project
	switch destination.kind {
	case taskContainerProject:
		found, findErr := NewProjects(s.db).Find(ctx, destination.id)
		if findErr != nil {
			return findErr
		}
		destinationProject = &found
	case taskContainerArea:
		if _, findErr := NewAreas(s.db).Find(ctx, destination.id); findErr != nil {
			return findErr
		}
	}

	var sourceProject *project.Project
	if source.kind == taskContainerProject {
		found, findErr := NewProjects(s.db).Find(ctx, source.id)
		if findErr != nil {
			return findErr
		}
		sourceProject = &found
	}

	destinationResolved := destinationProject != nil && destinationProject.Status != "open"
	sourceResolved := sourceProject != nil && sourceProject.Status != "open"
	switch {
	case sourceResolved && destinationResolved:
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf(
				"cannot move task %d from project %d to project %d while both projects are resolved; reopen both projects first",
				taskID,
				sourceProject.ID,
				destinationProject.ID,
			),
			cause,
		)
	case destinationResolved:
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf(
				"cannot move task %d into project %d while the project is resolved; reopen the project first",
				taskID,
				destinationProject.ID,
			),
			cause,
		)
	case sourceResolved:
		return apperr.New(
			apperr.Conflict,
			fmt.Sprintf(
				"cannot move task %d out of project %d while the project is resolved; reopen the project first",
				taskID,
				sourceProject.ID,
			),
			cause,
		)
	default:
		return fmt.Errorf("edit task: %w", cause)
	}
}

func taskContainerOf(current task.Task) taskContainer {
	switch {
	case current.ProjectID != nil:
		return taskContainer{kind: taskContainerProject, id: *current.ProjectID}
	case current.AreaID != nil:
		return taskContainer{kind: taskContainerArea, id: *current.AreaID}
	default:
		return taskContainer{kind: taskContainerInbox}
	}
}

func taskContainerAfterChange(current taskContainer, fields task.EditFields) taskContainer {
	if fields.Project.Set != nil {
		return taskContainer{kind: taskContainerProject, id: *fields.Project.Set}
	}
	if fields.Area.Set != nil {
		return taskContainer{kind: taskContainerArea, id: *fields.Area.Set}
	}
	if fields.Project.Clear && current.kind == taskContainerProject ||
		fields.Area.Clear && current.kind == taskContainerArea {
		return taskContainer{kind: taskContainerInbox}
	}

	return current
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

func collectContainedTasks(rows *sql.Rows, noun string, containerID int64) ([]task.Task, error) {
	defer func() {
		_ = rows.Close()
	}()

	containerExists := false
	tasks := make([]task.Task, 0)
	for rows.Next() {
		current, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed %s task: %w", noun, err)
		}
		containerExists = true
		if current.ID != 0 {
			tasks = append(tasks, current)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed %s tasks: %w", noun, err)
	}
	if !containerExists {
		return nil, apperr.New(
			apperr.NotFound,
			fmt.Sprintf("no %s %d", noun, containerID),
			sql.ErrNoRows,
		)
	}

	return tasks, nil
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

func collectViewTasks(rows *sql.Rows, scanAction, iterateAction string) ([]task.ViewTask, error) {
	defer func() {
		_ = rows.Close()
	}()

	tasks := make([]task.ViewTask, 0)
	for rows.Next() {
		current, err := scanViewTask(rows)
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
	err := scanner.Scan(taskScanTargets(&value)...)

	return value, err
}

func scanViewTask(scanner rowScanner) (task.ViewTask, error) {
	var value task.ViewTask
	targets := append(
		taskScanTargets(&value.Task),
		&value.ProjectTitle,
		&value.GoverningAreaID,
		&value.GoverningAreaTitle,
	)
	err := scanner.Scan(targets...)

	return value, err
}

func taskScanTargets(value *task.Task) []any {
	return []any{
		&value.ID,
		&value.ProjectID,
		&value.AreaID,
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
	}
}

func nullableID(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}
