package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmcampanini/gsd/internal/logbook"
)

type Logbook struct {
	db *DB
}

func NewLogbook(database *DB) *Logbook {
	return &Logbook{db: database}
}

func (s *Logbook) List(ctx context.Context) ([]logbook.Entry, error) {
	rows, err := s.db.database.QueryContext(ctx, `
SELECT kind, id, title, status, resolved_at, project_title
FROM logbook
ORDER BY resolved_at DESC,
         CASE kind WHEN 'project' THEN 0 ELSE 1 END,
         id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("query logbook: %w", err)
	}

	return collectLogbookEntries(rows)
}

func collectLogbookEntries(rows *sql.Rows) ([]logbook.Entry, error) {
	defer func() {
		_ = rows.Close()
	}()

	entries := make([]logbook.Entry, 0)
	for rows.Next() {
		var entry logbook.Entry
		if err := rows.Scan(
			&entry.Kind,
			&entry.ID,
			&entry.Title,
			&entry.Status,
			&entry.ResolvedAt,
			&entry.ProjectTitle,
		); err != nil {
			return nil, fmt.Errorf("scan logbook entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate logbook: %w", err)
	}

	return entries, nil
}
