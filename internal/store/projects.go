package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
)

const projectColumns = `id, title, note, done_at, cancelled_at, status, position, created_at, updated_at`

type Projects struct {
	db *DB
}

func NewProjects(database *DB) *Projects {
	return &Projects{db: database}
}

func (s *Projects) Add(
	ctx context.Context,
	fields project.AddFields,
	timestamp string,
) (project.Project, error) {
	row := s.db.database.QueryRowContext(ctx, `
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
	row := s.db.database.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", id)
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

	rows, err := s.db.database.QueryContext(ctx, query, arguments...)
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
	edited, err := scanProject(s.db.database.QueryRowContext(ctx, query, arguments...))
	if errors.Is(err, sql.ErrNoRows) {
		return project.Project{}, apperr.New(apperr.NotFound, fmt.Sprintf("no project %d", id), err)
	}
	if err != nil {
		return project.Project{}, fmt.Errorf("edit project: %w", err)
	}

	return edited, nil
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
