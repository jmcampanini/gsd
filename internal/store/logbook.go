package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmcampanini/gsd/internal/logbook"
)

type Logbook struct {
	database *DB
}

type logbookCore struct {
	executor logbookExecutor
}

type logbookExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func NewLogbook(database *DB) *Logbook {
	return &Logbook{database: database}
}

func (s *Logbook) List(ctx context.Context) ([]logbook.Entry, error) {
	return (&logbookCore{executor: s.database.database}).List(ctx)
}

func (s *logbookCore) List(ctx context.Context) ([]logbook.Entry, error) {
	rows, err := s.executor.QueryContext(ctx, `
SELECT kind, id, title, status, resolved_at, project_title,
       governing_area_id, governing_area_title, tags
FROM logbook
ORDER BY resolved_at DESC,
         CASE kind WHEN 'project' THEN 0 ELSE 1 END,
         id DESC
`)
	if err != nil {
		return nil, fmt.Errorf("query logbook: %w", err)
	}

	return collectRows(rows, scanLogbookEntry, "scan logbook entry", "iterate logbook")
}

func scanLogbookEntry(scanner rowScanner) (logbook.Entry, error) {
	var entry logbook.Entry
	err := scanner.Scan(
		&entry.Kind,
		&entry.ID,
		&entry.Title,
		&entry.Status,
		&entry.ResolvedAt,
		&entry.ProjectTitle,
		&entry.GoverningAreaID,
		&entry.GoverningAreaTitle,
		scanTagTitles(&entry.Tags),
	)

	return entry, err
}
