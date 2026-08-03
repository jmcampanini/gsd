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
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

const areaColumns = `id, title, note, archived_at, position, created_at, updated_at`

type Areas struct {
	database *DB
}

type areasCore struct {
	executor areaExecutor
}

type areaExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func NewAreas(database *DB) *Areas {
	return &Areas{database: database}
}

func (s *Areas) Add(
	ctx context.Context,
	fields area.AddFields,
	timestamp string,
) (area.Area, error) {
	return s.poolCore().Add(ctx, fields, timestamp)
}

func (s *Areas) Find(ctx context.Context, id int64) (area.Area, error) {
	return s.poolCore().Find(ctx, id)
}

func (s *Areas) List(ctx context.Context, options area.ListOptions) ([]area.Area, error) {
	return s.poolCore().List(ctx, options)
}

func (s *Areas) Edit(
	ctx context.Context,
	id int64,
	fields area.EditFields,
	timestamp string,
) (area.Area, error) {
	return s.poolCore().Edit(ctx, id, fields, timestamp)
}

func (s *Areas) Archive(
	ctx context.Context,
	id int64,
	timestamp string,
) (area.Area, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction area.Transaction) (area.Area, error) {
		return transaction.Archive(ctx, id, timestamp)
	})
}

func (s *Areas) Unarchive(
	ctx context.Context,
	id int64,
	timestamp string,
) (area.Area, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction area.Transaction) (area.Area, error) {
		return transaction.Unarchive(ctx, id, timestamp)
	})
}

func (s *Areas) Delete(ctx context.Context, id int64) (area.Area, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction area.Transaction) (area.Area, error) {
		return transaction.Delete(ctx, id)
	})
}

func (s *Areas) DeleteProjects(
	ctx context.Context,
	areaID int64,
) ([]project.Project, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction area.Transaction) ([]project.Project, error) {
		return transaction.DeleteProjects(ctx, areaID)
	})
}

func (s *Areas) DeleteTasks(
	ctx context.Context,
	areaID int64,
	scope area.TaskDeletionScope,
) ([]task.Task, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction area.Transaction) ([]task.Task, error) {
		return transaction.DeleteTasks(ctx, areaID, scope)
	})
}

func (s *Areas) ResolveTags(ctx context.Context, names []string) ([]tag.Tag, error) {
	if len(names) <= 1 {
		return s.poolCore().ResolveTags(ctx, names)
	}

	return runInTransaction(ctx, s.withinReadTransaction, func(transaction area.Transaction) ([]tag.Tag, error) {
		return transaction.ResolveTags(ctx, names)
	})
}

func (s *Areas) AttachTags(ctx context.Context, areaID int64, tags []tag.Tag) error {
	if len(tags) <= 1 {
		return s.poolCore().AttachTags(ctx, areaID, tags)
	}
	return s.WithinTransaction(ctx, func(transaction area.Transaction) error {
		return transaction.AttachTags(ctx, areaID, tags)
	})
}

func (s *Areas) DetachTags(ctx context.Context, areaID int64, tags []tag.Tag) error {
	if len(tags) <= 1 {
		return s.poolCore().DetachTags(ctx, areaID, tags)
	}
	return s.WithinTransaction(ctx, func(transaction area.Transaction) error {
		return transaction.DetachTags(ctx, areaID, tags)
	})
}

func (s *Areas) WithinTransaction(
	ctx context.Context,
	apply func(area.Transaction) error,
) error {
	return withinImmediateTransaction(ctx, s.database, "area", func(connection *sql.Conn) error {
		return apply(&areasCore{executor: connection})
	})
}

func (s *Areas) withinReadTransaction(
	ctx context.Context,
	apply func(area.Transaction) error,
) error {
	return withinDeferredTransaction(ctx, s.database, "area", func(connection *sql.Conn) error {
		return apply(&areasCore{executor: connection})
	})
}

func (s *Areas) poolCore() *areasCore {
	return &areasCore{executor: s.database.database}
}

func (s *areasCore) Add(
	ctx context.Context,
	fields area.AddFields,
	timestamp string,
) (area.Area, error) {
	created, err := scanArea(s.executor.QueryRowContext(ctx, `
INSERT INTO areas (title, note, position, created_at, updated_at)
SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
FROM areas
RETURNING `+areaColumnsWithTags("areas.id"), fields.Title, fields.Note, timestamp, timestamp))
	if err != nil {
		return area.Area{}, fmt.Errorf("insert area: %w", err)
	}

	return created, nil
}

func (s *areasCore) Find(ctx context.Context, id int64) (area.Area, error) {
	found, err := scanArea(s.executor.QueryRowContext(
		ctx,
		"SELECT "+areaColumnsWithTags("areas.id")+" FROM areas WHERE id = ?",
		id,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return area.Area{}, apperr.New(apperr.NotFound, fmt.Sprintf("no area %d", id), err)
	}
	if err != nil {
		return area.Area{}, fmt.Errorf("find area: %w", err)
	}

	return found, nil
}

func (s *areasCore) List(ctx context.Context, options area.ListOptions) ([]area.Area, error) {
	query := "SELECT " + areaColumnsWithTags("areas.id") + " FROM areas"
	switch options.Slice {
	case area.ListSliceActive:
		query += " WHERE archived_at IS NULL"
	case area.ListSliceArchived:
		query += " WHERE archived_at IS NOT NULL"
	case area.ListSliceAll:
	default:
		return nil, fmt.Errorf("invalid area list slice %q", options.Slice)
	}
	query += " ORDER BY position, id"

	rows, err := s.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list areas: %w", err)
	}

	return collectRows(rows, scanArea, "scan listed area", "iterate listed areas")
}

func (s *areasCore) Edit(
	ctx context.Context,
	id int64,
	fields area.EditFields,
	timestamp string,
) (area.Area, error) {
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
		return area.Area{}, errors.New("area edit requires at least one field")
	}

	assignments = append(assignments, "updated_at = ?")
	arguments = append(arguments, timestamp, id)
	query := "UPDATE areas SET " + strings.Join(assignments, ", ") +
		" WHERE id = ? RETURNING " + areaColumnsWithTags("areas.id")
	edited, err := scanArea(s.executor.QueryRowContext(ctx, query, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return area.Area{}, apperr.New(apperr.NotFound, fmt.Sprintf("no area %d", id), err)
	}
	if err != nil {
		return area.Area{}, fmt.Errorf("edit area: %w", err)
	}

	return edited, nil
}

func (s *areasCore) Archive(
	ctx context.Context,
	id int64,
	timestamp string,
) (area.Area, error) {
	archived, err := scanArea(s.executor.QueryRowContext(ctx, `
UPDATE areas
SET archived_at = ?, updated_at = ?
WHERE id = ? AND archived_at IS NULL
RETURNING `+areaColumnsWithTags("areas.id"), timestamp, timestamp, id))
	if err == nil {
		return archived, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return area.Area{}, fmt.Errorf("archive area: %w", err)
	}
	if _, findErr := s.Find(ctx, id); findErr != nil {
		return area.Area{}, findErr
	}

	return area.Area{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot archive area %d while it is already archived", id),
		err,
	)
}

func (s *areasCore) Unarchive(
	ctx context.Context,
	id int64,
	timestamp string,
) (area.Area, error) {
	unarchived, err := scanArea(s.executor.QueryRowContext(ctx, `
UPDATE areas
SET archived_at = NULL, updated_at = ?
WHERE id = ? AND archived_at IS NOT NULL
RETURNING `+areaColumnsWithTags("areas.id"), timestamp, id))
	if err == nil {
		return unarchived, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return area.Area{}, fmt.Errorf("unarchive area: %w", err)
	}
	if _, findErr := s.Find(ctx, id); findErr != nil {
		return area.Area{}, findErr
	}

	return area.Area{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("cannot unarchive area %d while it is active", id),
		err,
	)
}

func (s *areasCore) Delete(ctx context.Context, id int64) (area.Area, error) {
	deleted, err := scanArea(s.executor.QueryRowContext(ctx, `
SELECT `+areaColumnsWithTags("areas.id")+`
FROM areas
WHERE id = ?
  AND NOT EXISTS (SELECT 1 FROM projects WHERE area_id = areas.id)
  AND NOT EXISTS (SELECT 1 FROM tasks WHERE area_id = areas.id)
`, id))
	if errors.Is(err, sql.ErrNoRows) {
		if _, findErr := s.Find(ctx, id); findErr != nil {
			return area.Area{}, findErr
		}
		return area.Area{}, apperr.New(
			apperr.Conflict,
			fmt.Sprintf("cannot delete area %d while it contains projects or tasks", id),
			err,
		)
	}
	if err != nil {
		return area.Area{}, fmt.Errorf("delete area: %w", err)
	}

	if err := deleteRows(ctx, s.executor, 1, "DELETE FROM areas WHERE id = ?", id); err != nil {
		return area.Area{}, fmt.Errorf("delete area: %w", err)
	}
	return deleted, nil
}

func (s *areasCore) DeleteProjects(
	ctx context.Context,
	areaID int64,
) ([]project.Project, error) {
	rows, err := s.executor.QueryContext(
		ctx,
		"SELECT "+projectColumnsWithTags("projects.id")+" FROM projects WHERE area_id = ?",
		areaID,
	)
	if err != nil {
		return nil, fmt.Errorf("delete area projects: %w", err)
	}
	deleted, err := collectRows(rows, scanProject, "scan listed project", "iterate listed projects")
	if err != nil {
		return nil, err
	}

	if err := deleteRows(
		ctx,
		s.executor,
		int64(len(deleted)),
		"DELETE FROM projects WHERE area_id = ?",
		areaID,
	); err != nil {
		return nil, fmt.Errorf("delete area projects: %w", err)
	}
	sort.Slice(deleted, func(left, right int) bool {
		if deleted[left].Position != deleted[right].Position {
			return deleted[left].Position < deleted[right].Position
		}
		return deleted[left].ID < deleted[right].ID
	})

	return deleted, nil
}

func (s *areasCore) DeleteTasks(
	ctx context.Context,
	areaID int64,
	scope area.TaskDeletionScope,
) ([]task.Task, error) {
	var condition string
	switch scope {
	case area.TaskDeletionScopeProject:
		condition = "project_id IN (SELECT id FROM projects WHERE area_id = ?)"
	case area.TaskDeletionScopeLoose:
		condition = "area_id = ?"
	default:
		return nil, fmt.Errorf("invalid area task deletion scope %q", scope)
	}

	rows, err := s.executor.QueryContext(
		ctx,
		"SELECT "+taskColumnsWithTags("tasks.id")+" FROM tasks WHERE "+condition,
		areaID,
	)
	if err != nil {
		return nil, fmt.Errorf("delete area tasks: %w", err)
	}
	deleted, err := collectRows(rows, scanTask, "scan deleted area task", "iterate deleted area tasks")
	if err != nil {
		return nil, err
	}

	if err := deleteRows(
		ctx,
		s.executor,
		int64(len(deleted)),
		"DELETE FROM tasks WHERE "+condition,
		areaID,
	); err != nil {
		return nil, fmt.Errorf("delete area tasks: %w", err)
	}
	sortTasks(deleted)

	return deleted, nil
}

func (s *areasCore) ResolveTags(ctx context.Context, names []string) ([]tag.Tag, error) {
	return resolveStoredTags(ctx, s.executor, names)
}

func (s *areasCore) AttachTags(ctx context.Context, areaID int64, tags []tag.Tag) error {
	return attachEntityTags(ctx, s.executor, areaTagSpec, areaID, tags)
}

func (s *areasCore) DetachTags(ctx context.Context, areaID int64, tags []tag.Tag) error {
	return detachEntityTags(ctx, s.executor, areaTagSpec, areaID, tags)
}

func scanArea(scanner rowScanner) (area.Area, error) {
	var value area.Area
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.Note,
		&value.ArchivedAt,
		&value.Position,
		&value.CreatedAt,
		&value.UpdatedAt,
		scanTagTitles(&value.Tags),
	)

	return value, err
}

func areaColumnsWithTags(entityReference string) string {
	return areaColumns + ", " + tagJSONExpression(areaTagSpec, entityReference) + " AS tags"
}
