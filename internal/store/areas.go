package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
)

const areaColumns = `id, title, note, archived_at, position, created_at, updated_at`

type Areas struct {
	executor areaExecutor
}

type areaExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewAreas(database *DB) *Areas {
	return &Areas{executor: database.database}
}

func (s *Areas) Add(
	ctx context.Context,
	fields area.AddFields,
	timestamp string,
) (area.Area, error) {
	created, err := scanArea(s.executor.QueryRowContext(ctx, `
INSERT INTO areas (title, note, position, created_at, updated_at)
SELECT ?, ?, COALESCE(MAX(position), -1) + 1, ?, ?
FROM areas
RETURNING `+areaColumns, fields.Title, fields.Note, timestamp, timestamp))
	if err != nil {
		return area.Area{}, fmt.Errorf("insert area: %w", err)
	}

	return created, nil
}

func (s *Areas) Find(ctx context.Context, id int64) (area.Area, error) {
	found, err := scanArea(s.executor.QueryRowContext(
		ctx,
		"SELECT "+areaColumns+" FROM areas WHERE id = ?",
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

func (s *Areas) List(ctx context.Context, options area.ListOptions) ([]area.Area, error) {
	query := "SELECT " + areaColumns + " FROM areas"
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

	return collectAreas(rows)
}

func (s *Areas) Edit(
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
		" WHERE id = ? RETURNING " + areaColumns
	edited, err := scanArea(s.executor.QueryRowContext(ctx, query, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return area.Area{}, apperr.New(apperr.NotFound, fmt.Sprintf("no area %d", id), err)
	}
	if err != nil {
		return area.Area{}, fmt.Errorf("edit area: %w", err)
	}

	return edited, nil
}

func collectAreas(rows *sql.Rows) ([]area.Area, error) {
	defer func() {
		_ = rows.Close()
	}()

	areas := make([]area.Area, 0)
	for rows.Next() {
		current, err := scanArea(rows)
		if err != nil {
			return nil, fmt.Errorf("scan listed area: %w", err)
		}
		areas = append(areas, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed areas: %w", err)
	}

	return areas, nil
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
	)

	return value, err
}
