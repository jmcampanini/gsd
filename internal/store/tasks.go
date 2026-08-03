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
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

const taskColumns = `id, project_id, area_id, title, note, defer_until, due_on, done_at, cancelled_at, status, position, created_at, updated_at`

const taskViewColumns = taskColumns + `, project_title, governing_area_id, governing_area_title, tags`

type Tasks struct {
	database *DB
}

type tasksCore struct {
	executor taskExecutor
}

type taskExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
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
	return &Tasks{database: database}
}

func (s *Tasks) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Add(ctx, fields, timestamp)
	})
}

func (s *Tasks) Inbox(ctx context.Context) ([]task.ViewTask, error) {
	return (&tasksCore{executor: s.database.database}).Inbox(ctx)
}

func (s *Tasks) Available(ctx context.Context) ([]task.ViewTask, error) {
	return (&tasksCore{executor: s.database.database}).Available(ctx)
}

func (s *Tasks) Find(ctx context.Context, id int64) (task.Task, error) {
	return (&tasksCore{executor: s.database.database}).Find(ctx, id)
}

func (s *Tasks) List(ctx context.Context, filter task.ListFilter) ([]task.Task, error) {
	return (&tasksCore{executor: s.database.database}).List(ctx, filter)
}

func (s *Tasks) ProjectExists(ctx context.Context, id int64) error {
	return (&tasksCore{executor: s.database.database}).ProjectExists(ctx, id)
}

func (s *Tasks) AreaExists(ctx context.Context, id int64) error {
	return (&tasksCore{executor: s.database.database}).AreaExists(ctx, id)
}

func (s *Tasks) Edit(ctx context.Context, id int64, fields task.EditFields, timestamp string) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Edit(ctx, id, fields, timestamp)
	})
}

func (s *Tasks) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Done(ctx, id, timestamp)
	})
}

func (s *Tasks) Cancel(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Cancel(ctx, id, timestamp)
	})
}

func (s *Tasks) Reopen(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Reopen(ctx, id, timestamp)
	})
}

func (s *Tasks) Delete(ctx context.Context, id int64) (task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction task.Transaction) (task.Task, error) {
		return transaction.Delete(ctx, id)
	})
}

func (s *Tasks) ResolveTags(ctx context.Context, names []string) ([]tag.Tag, error) {
	return runInTransaction(ctx, s.WithinReadTransaction, func(transaction task.Transaction) ([]tag.Tag, error) {
		return transaction.ResolveTags(ctx, names)
	})
}

func (s *Tasks) AttachTags(ctx context.Context, taskID int64, tags []tag.Tag) error {
	return s.WithinTransaction(ctx, func(transaction task.Transaction) error {
		return transaction.AttachTags(ctx, taskID, tags)
	})
}

func (s *Tasks) DetachTags(ctx context.Context, taskID int64, tags []tag.Tag) error {
	return s.WithinTransaction(ctx, func(transaction task.Transaction) error {
		return transaction.DetachTags(ctx, taskID, tags)
	})
}

func (s *Tasks) WithinTransaction(ctx context.Context, apply func(task.Transaction) error) error {
	return withinImmediateTransaction(ctx, s.database, "task", func(connection *sql.Conn) error {
		return apply(&tasksCore{executor: connection})
	})
}

func (s *Tasks) WithinReadTransaction(ctx context.Context, apply func(task.Transaction) error) error {
	return withinDeferredTransaction(ctx, s.database, "task", func(connection *sql.Conn) error {
		return apply(&tasksCore{executor: connection})
	})
}

func (s *tasksCore) Add(ctx context.Context, fields task.AddFields, timestamp string) (task.Task, error) {
	if fields.ProjectID != nil && fields.AreaID != nil {
		return task.Task{}, errors.New("task cannot have both project and area")
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
RETURNING `+taskColumnsWithTags("tasks.id"),
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

func (s *tasksCore) Inbox(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.executor.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM inbox ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}

	return collectRows(rows, scanViewTask, "scan inbox task", "iterate inbox")
}

func (s *tasksCore) Available(ctx context.Context) ([]task.ViewTask, error) {
	rows, err := s.executor.QueryContext(ctx, "SELECT "+taskViewColumns+" FROM available ORDER BY position, id")
	if err != nil {
		return nil, fmt.Errorf("query available tasks: %w", err)
	}

	return collectRows(rows, scanViewTask, "scan available task", "iterate available tasks")
}

func (s *tasksCore) Find(ctx context.Context, id int64) (task.Task, error) {
	row := s.executor.QueryRowContext(
		ctx,
		"SELECT "+taskColumnsWithTags("tasks.id")+" FROM tasks WHERE id = ?",
		id,
	)
	found, err := scanTask(row)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, apperr.New(apperr.NotFound, fmt.Sprintf("no task %d", id), err)
	}
	if err != nil {
		return task.Task{}, fmt.Errorf("find task: %w", err)
	}

	return found, nil
}

func (s *tasksCore) List(ctx context.Context, filter task.ListFilter) ([]task.Task, error) {
	if filter.ProjectID != nil && filter.AreaID != nil {
		return nil, errors.New("task list cannot filter by both project and area")
	}

	conditions := make([]string, 0, 5)
	arguments := make([]any, 0, 4)
	switch filter.Status {
	case task.ListStatusOpen, task.ListStatusDone, task.ListStatusCancelled:
		conditions = append(conditions, "status = ?")
		arguments = append(arguments, filter.Status)
	case task.ListStatusAll:
	default:
		return nil, fmt.Errorf("invalid list status %q", filter.Status)
	}

	switch filter.Date {
	case task.DateSelectorNone:
	case task.DateSelectorDue:
		conditions = append(conditions, "due_on IS NOT NULL")
	case task.DateSelectorOverdue:
		conditions = append(conditions, "status = 'open' AND due_on < date('now', 'localtime')")
	case task.DateSelectorDeferred:
		conditions = append(conditions, "defer_until > date('now', 'localtime')")
	default:
		return nil, fmt.Errorf("invalid date selector %q", filter.Date)
	}

	if filter.ProjectID != nil {
		conditions = append(conditions, "project_id = ?")
		arguments = append(arguments, *filter.ProjectID)
	}
	if filter.AreaID != nil {
		conditions = append(conditions, "area_id = ?")
		arguments = append(arguments, *filter.AreaID)
	}
	if filter.TagID != nil {
		conditions = append(conditions, `EXISTS (
    SELECT 1 FROM task_tags
    WHERE task_id = tasks.id AND tag_id = ?
)`)
		arguments = append(arguments, *filter.TagID)
	}

	query := "SELECT " + taskColumnsWithTags("tasks.id") + " FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY position, id"

	rows, err := s.executor.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	return collectRows(rows, scanTask, "scan listed task", "iterate listed tasks")
}

func (s *tasksCore) ProjectExists(ctx context.Context, id int64) error {
	_, err := (&projectsCore{executor: s.executor}).Find(ctx, id)
	return err
}

func (s *tasksCore) AreaExists(ctx context.Context, id int64) error {
	_, err := (&areasCore{executor: s.executor}).Find(ctx, id)
	return err
}

func (s *tasksCore) Edit(
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
		"UPDATE tasks SET "+strings.Join(assignments, ", ")+" WHERE id = ? RETURNING "+
			taskColumnsWithTags("tasks.id"),
		arguments...,
	))
	if err != nil {
		return task.Task{}, fmt.Errorf("edit task: %w", err)
	}

	return edited, nil
}

func (s *tasksCore) Done(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "complete", timestamp)
}

func (s *tasksCore) Cancel(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "cancel", timestamp)
}

func (s *tasksCore) Reopen(ctx context.Context, id int64, timestamp string) (task.Task, error) {
	return s.applyTransition(ctx, id, "reopen", timestamp)
}

func (s *tasksCore) applyTransition(
	ctx context.Context,
	id int64,
	action string,
	timestamp string,
) (task.Task, error) {
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
		query = "UPDATE tasks SET done_at = ?, updated_at = ? WHERE id = ? RETURNING " +
			taskColumnsWithTags("tasks.id")
		arguments = []any{timestamp, timestamp, id}
	case "cancel":
		valid = current.DoneAt == nil && current.CancelledAt == nil
		query = "UPDATE tasks SET cancelled_at = ?, updated_at = ? WHERE id = ? RETURNING " +
			taskColumnsWithTags("tasks.id")
		arguments = []any{timestamp, timestamp, id}
	case "reopen":
		valid = current.DoneAt != nil || current.CancelledAt != nil
		query = "UPDATE tasks SET done_at = NULL, cancelled_at = NULL, updated_at = ? WHERE id = ? RETURNING " +
			taskColumnsWithTags("tasks.id")
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

func (s *tasksCore) Delete(ctx context.Context, id int64) (task.Task, error) {
	found, err := s.Find(ctx, id)
	if err != nil {
		return task.Task{}, err
	}
	if err := deleteRows(ctx, s.executor, 1, "DELETE FROM tasks WHERE id = ?", id); err != nil {
		return task.Task{}, fmt.Errorf("delete task: %w", err)
	}

	return found, nil
}

func taskColumnsWithTags(entityReference string) string {
	return taskColumns + ", " + tagJSONExpression(taskTagSpec, entityReference) + " AS tags"
}

func (s *tasksCore) ResolveTags(ctx context.Context, names []string) ([]tag.Tag, error) {
	return resolveStoredTags(ctx, s.executor, names)
}

func (s *tasksCore) AttachTags(ctx context.Context, taskID int64, tags []tag.Tag) error {
	return attachEntityTags(ctx, s.executor, taskTagSpec, taskID, tags)
}

func (s *tasksCore) DetachTags(ctx context.Context, taskID int64, tags []tag.Tag) error {
	return detachEntityTags(ctx, s.executor, taskTagSpec, taskID, tags)
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

func (s *tasksCore) findContainerState(
	ctx context.Context,
	container taskContainer,
) (taskContainerState, error) {
	var state taskContainerState
	switch container.kind {
	case taskContainerInbox:
	case taskContainerProject:
		found, err := (&projectsCore{executor: s.executor}).Find(ctx, container.id)
		if err != nil {
			return taskContainerState{}, err
		}
		state.project = &found
		if found.AreaID != nil {
			governingArea, areaErr := (&areasCore{executor: s.executor}).Find(ctx, *found.AreaID)
			if areaErr != nil {
				return taskContainerState{}, areaErr
			}
			state.area = &governingArea
		}
	case taskContainerArea:
		found, err := (&areasCore{executor: s.executor}).Find(ctx, container.id)
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

func scanTask(scanner rowScanner) (task.Task, error) {
	var value task.Task
	targets := append(taskBaseScanTargets(&value), scanTagTitles(&value.Tags))
	err := scanner.Scan(targets...)

	return value, err
}

func scanViewTask(scanner rowScanner) (task.ViewTask, error) {
	var value task.ViewTask
	targets := append(
		taskBaseScanTargets(&value.Task),
		&value.ProjectTitle,
		&value.GoverningAreaID,
		&value.GoverningAreaTitle,
		scanTagTitles(&value.Tags),
	)
	err := scanner.Scan(targets...)

	return value, err
}

func taskBaseScanTargets(value *task.Task) []any {
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
