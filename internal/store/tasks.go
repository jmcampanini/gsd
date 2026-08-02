package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const taskColumns = `id, project_id, area_id, title, note, defer_until, due_on, done_at, cancelled_at, status, position, created_at, updated_at`

const taskViewColumns = taskColumns + `, project_title, governing_area_id, governing_area_title`

type Tasks struct {
	database *DB
	executor taskExecutor
}

type taskExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
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

type taskContainerState struct {
	project *project.Project
	area    *area.Area
}

func NewTasks(database *DB) *Tasks {
	return &Tasks{database: database, executor: database.database}
}

func (s *Tasks) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	if fields.ProjectID != nil && fields.AreaID != nil {
		return task.Task{}, errors.New("task cannot have both project and area")
	}
	if s.database != nil {
		var created task.Task
		err := s.WithinTransaction(ctx, func(transaction task.Store) error {
			var operationErr error
			created, operationErr = transaction.Add(ctx, fields, timestamp)
			return operationErr
		})
		if err != nil {
			return task.Task{}, err
		}
		return created, nil
	}

	container := taskContainerForAdd(fields)
	state, err := s.findContainerState(ctx, container)
	if err != nil {
		return task.Task{}, err
	}
	resolvedProjectIDs, archivedAreaIDs := taskContainerBlockers(state)
	if len(resolvedProjectIDs) > 0 || len(archivedAreaIDs) > 0 {
		return task.Task{}, taskBlockersConflict(
			taskAddAction(container),
			resolvedProjectIDs,
			archivedAreaIDs,
			nil,
		)
	}

	projectID := nullableID(fields.ProjectID)
	areaID := nullableID(fields.AreaID)
	created, err := scanTask(s.executor.QueryRowContext(ctx, `
INSERT INTO tasks (
    project_id, area_id, title, note, defer_until, due_on, position, created_at, updated_at
)
VALUES (
    ?, ?, ?, ?, ?, ?,
    (
        SELECT COALESCE(MAX(position), -1) + 1
        FROM tasks
        WHERE project_id IS ? AND area_id IS ?
    ),
    ?, ?
)
RETURNING `+taskColumns,
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
	))
	if err != nil {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}

	return created, nil
}

func (s *Tasks) Inbox(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.executor.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM inbox ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}

	return collectViewTasks(rows, "scan inbox task", "iterate inbox")
}

func (s *Tasks) Available(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.executor.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM available ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query available tasks: %w", err)
	}

	return collectViewTasks(rows, "scan available task", "iterate available tasks")
}

func (s *Tasks) Find(ctx context.Context, id int64) (task.Task, error) {
	row := s.executor.QueryRowContext(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = ?", id)
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
		conditions = append(conditions, "status = 'open' AND due_on < date('now', 'localtime')")
	case task.DateSelectorDeferred:
		conditions = append(conditions, "defer_until > date('now', 'localtime')")
	default:
		return nil, fmt.Errorf("invalid date selector %q", options.Date)
	}

	if options.ProjectID != nil || options.AreaID != nil {
		if s.database != nil {
			var listed []task.Task
			err := s.WithinTransaction(ctx, func(transaction task.Store) error {
				var operationErr error
				listed, operationErr = transaction.List(ctx, options)
				return operationErr
			})
			if err != nil {
				return nil, err
			}
			return listed, nil
		}
		if options.ProjectID != nil {
			return s.listContained(ctx, taskContainerProject, *options.ProjectID, conditions, arguments)
		}
		return s.listContained(ctx, taskContainerArea, *options.AreaID, conditions, arguments)
	}

	query := "SELECT " + taskColumns + " FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY position, id"

	rows, err := s.executor.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	return collectTasks(rows, "scan listed task", "iterate listed tasks")
}

func (s *Tasks) listContained(
	ctx context.Context,
	kind taskContainerKind,
	containerID int64,
	conditions []string,
	arguments []any,
) ([]task.Task, error) {
	var column string
	var noun string
	switch kind {
	case taskContainerProject:
		column = "project_id"
		noun = "project"
		if _, err := (&Projects{executor: s.executor}).Find(ctx, containerID); err != nil {
			return nil, err
		}
	case taskContainerArea:
		column = "area_id"
		noun = "area"
		if _, err := (&Areas{executor: s.executor}).Find(ctx, containerID); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid task container kind %d", kind)
	}

	query := "SELECT " + taskColumns + " FROM tasks WHERE " + column + " = ?"
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY position, id"

	scopedArguments := make([]any, 0, len(arguments)+1)
	scopedArguments = append(scopedArguments, containerID)
	scopedArguments = append(scopedArguments, arguments...)
	rows, err := s.executor.QueryContext(ctx, query, scopedArguments...)
	if err != nil {
		return nil, fmt.Errorf("list %s tasks: %w", noun, err)
	}

	return collectTasks(rows, "scan listed "+noun+" task", "iterate listed "+noun+" tasks")
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

	contentChanged := fields.Title != nil || fields.Note != nil ||
		fields.DueOn.Set != nil || fields.DueOn.Clear ||
		fields.DeferUntil.Set != nil || fields.DeferUntil.Clear
	membershipRequested := fields.Project.Set != nil || fields.Project.Clear ||
		fields.Area.Set != nil || fields.Area.Clear
	if !contentChanged && !membershipRequested {
		return task.Task{}, errors.New("edit requires at least one field")
	}
	if s.database != nil {
		var edited task.Task
		err := s.WithinTransaction(ctx, func(transaction task.Store) error {
			var operationErr error
			edited, operationErr = transaction.Edit(ctx, id, fields, timestamp)
			return operationErr
		})
		if err != nil {
			return task.Task{}, err
		}
		return edited, nil
	}

	current, err := s.Find(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	source := taskContainerOf(current)
	destination := taskContainerAfterChange(source, fields)
	moving := source != destination
	if moving {
		destinationState, destinationErr := s.findContainerState(ctx, destination)
		if destinationErr != nil {
			return task.Task{}, destinationErr
		}
		sourceState, sourceErr := s.findContainerState(ctx, source)
		if sourceErr != nil {
			return task.Task{}, sourceErr
		}

		destinationResolved, destinationArchived := taskContainerBlockers(destinationState)
		sourceResolved, sourceArchived := taskContainerBlockers(sourceState)
		resolvedProjectIDs := append(sourceResolved, destinationResolved...)
		archivedAreaIDs := append(sourceArchived, destinationArchived...)
		if len(resolvedProjectIDs) > 0 || len(archivedAreaIDs) > 0 {
			return task.Task{}, taskBlockersConflict(
				fmt.Sprintf("move task %d", id),
				resolvedProjectIDs,
				archivedAreaIDs,
				nil,
			)
		}
	}
	if !contentChanged && !moving {
		return current, nil
	}

	assignments := make([]string, 0, 10)
	arguments := make([]any, 0, 12)
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
	if moving {
		projectID, areaID := taskContainerIDs(destination)
		assignments = append(
			assignments,
			`position = (
    SELECT COALESCE(MAX(sibling.position), -1) + 1
    FROM tasks AS sibling
    WHERE sibling.project_id IS ? AND sibling.area_id IS ?
)`,
			"project_id = ?",
			"area_id = ?",
		)
		arguments = append(arguments, projectID, areaID, projectID, areaID)
	}
	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, id)

	edited, err := scanTask(s.executor.QueryRowContext(
		ctx,
		"UPDATE tasks SET "+strings.Join(assignments, ", ")+" WHERE id = ? RETURNING "+taskColumns,
		arguments...,
	))
	if err != nil {
		return task.Task{}, fmt.Errorf("edit task: %w", err)
	}

	return edited, nil
}

func (s *Tasks) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "complete", timestamp)
}

func (s *Tasks) Cancel(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "cancel", timestamp)
}

func (s *Tasks) Reopen(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "reopen", timestamp)
}

func (s *Tasks) applyTransition(
	ctx context.Context,
	id int64,
	action string,
	timestamp string,
) (task.Task, error) {
	if s.database != nil {
		var updated task.Task
		err := s.WithinTransaction(ctx, func(transaction task.Store) error {
			var operationErr error
			switch action {
			case "complete":
				updated, operationErr = transaction.Done(ctx, id, timestamp)
			case "cancel":
				updated, operationErr = transaction.Cancel(ctx, id, timestamp)
			case "reopen":
				updated, operationErr = transaction.Reopen(ctx, id, timestamp)
			default:
				operationErr = fmt.Errorf("invalid task transition %q", action)
			}
			return operationErr
		})
		if err != nil {
			return task.Task{}, err
		}
		return updated, nil
	}

	current, err := s.Find(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	state, err := s.findContainerState(ctx, taskContainerOf(current))
	if err != nil {
		return task.Task{}, err
	}
	resolvedProjectIDs, archivedAreaIDs := taskContainerBlockers(state)
	if len(resolvedProjectIDs) > 0 || len(archivedAreaIDs) > 0 {
		return task.Task{}, taskBlockersConflict(
			fmt.Sprintf("%s task %d", action, id),
			resolvedProjectIDs,
			archivedAreaIDs,
			nil,
		)
	}

	valid := false
	var query string
	var arguments []any
	switch action {
	case "complete":
		valid = current.DoneAt == nil && current.CancelledAt == nil
		query = "UPDATE tasks SET done_at = ?, updated_at = ? WHERE id = ? RETURNING " + taskColumns
		arguments = []any{timestamp, timestamp, id}
	case "cancel":
		valid = current.DoneAt == nil && current.CancelledAt == nil
		query = "UPDATE tasks SET cancelled_at = ?, updated_at = ? WHERE id = ? RETURNING " + taskColumns
		arguments = []any{timestamp, timestamp, id}
	case "reopen":
		valid = current.DoneAt != nil || current.CancelledAt != nil
		query = "UPDATE tasks SET done_at = NULL, cancelled_at = NULL, updated_at = ? WHERE id = ? RETURNING " + taskColumns
		arguments = []any{timestamp, id}
	default:
		return task.Task{}, fmt.Errorf("invalid task transition %q", action)
	}
	if !valid {
		return task.Task{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("cannot %s task %d in its current state", action, id),
			nil,
		)
	}

	updated, err := scanTask(s.executor.QueryRowContext(ctx, query, arguments...))
	if err != nil {
		return task.Task{}, fmt.Errorf("%s task: %w", action, err)
	}
	return updated, nil
}

func (s *Tasks) Delete(ctx context.Context, id int64) (task.Task, error) {
	row := s.executor.QueryRowContext(ctx, "DELETE FROM tasks WHERE id = ? RETURNING "+taskColumns, id)
	deleted, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, apperr.New(apperr.NotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("delete task: %w", err)
	}

	return deleted, nil
}

func (s *Tasks) WithinTransaction(
	ctx context.Context,
	apply func(task.Store) error,
) error {
	if s.database == nil {
		return errors.New("nested task transactions are not supported")
	}

	return withinImmediateTransaction(ctx, s.database, "task", func(connection *sql.Conn) error {
		return apply(&Tasks{executor: connection})
	})
}

func taskContainerForAdd(fields task.AddFields) taskContainer {
	switch {
	case fields.ProjectID != nil:
		return taskContainer{kind: taskContainerProject, id: *fields.ProjectID}
	case fields.AreaID != nil:
		return taskContainer{kind: taskContainerArea, id: *fields.AreaID}
	default:
		return taskContainer{kind: taskContainerInbox}
	}
}

func taskAddAction(container taskContainer) string {
	switch container.kind {
	case taskContainerProject:
		return fmt.Sprintf("add task to project %d", container.id)
	case taskContainerArea:
		return fmt.Sprintf("add task to area %d", container.id)
	default:
		return "add task"
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

func taskContainerIDs(container taskContainer) (any, any) {
	switch container.kind {
	case taskContainerInbox:
		return nil, nil
	case taskContainerProject:
		return container.id, nil
	case taskContainerArea:
		return nil, container.id
	default:
		panic(fmt.Sprintf("invalid task container kind %d", container.kind))
	}
}

func (s *Tasks) findContainerState(
	ctx context.Context,
	container taskContainer,
) (taskContainerState, error) {
	var state taskContainerState
	switch container.kind {
	case taskContainerInbox:
	case taskContainerProject:
		found, err := (&Projects{executor: s.executor}).Find(ctx, container.id)
		if err != nil {
			return taskContainerState{}, err
		}
		state.project = &found
		if found.AreaID != nil {
			governingArea, areaErr := (&Areas{executor: s.executor}).Find(ctx, *found.AreaID)
			if areaErr != nil {
				return taskContainerState{}, areaErr
			}
			state.area = &governingArea
		}
	case taskContainerArea:
		found, err := (&Areas{executor: s.executor}).Find(ctx, container.id)
		if err != nil {
			return taskContainerState{}, err
		}
		state.area = &found
	default:
		return taskContainerState{}, fmt.Errorf("invalid task container kind %d", container.kind)
	}

	return state, nil
}

func taskContainerBlockers(state taskContainerState) ([]int64, []int64) {
	resolvedProjectIDs := make([]int64, 0, 1)
	if state.project != nil && state.project.Status != "open" {
		resolvedProjectIDs = append(resolvedProjectIDs, state.project.ID)
	}
	archivedAreaIDs := make([]int64, 0, 1)
	if state.area != nil && state.area.ArchivedAt != nil {
		archivedAreaIDs = append(archivedAreaIDs, state.area.ID)
	}

	return resolvedProjectIDs, archivedAreaIDs
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
