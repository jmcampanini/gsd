package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tag"
)

const (
	tagColumns              = `id, title, created_at, updated_at`
	tagUsageCountExpression = `(SELECT COUNT(*) FROM task_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM project_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM area_tags WHERE tag_id = tags.id)`
)

// Tags implements tag.Store on SQLite. Titles compare with COLLATE NOCASE.
type Tags struct {
	database *DB
}

type tagsCore struct {
	executor tagExecutor
}

type tagExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// NewTags returns the tag store for database.
func NewTags(database *DB) *Tags {
	return &Tags{database: database}
}

// Add inserts a tag; a title already present in any casing is a conflict
// error naming the stored spelling.
func (s *Tags) Add(ctx context.Context, name, timestamp string) (tag.Tag, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction tag.Transaction) (tag.Tag, error) {
		return transaction.Add(ctx, name, timestamp)
	})
}

func (s *tagsCore) Add(ctx context.Context, name, timestamp string) (tag.Tag, error) {
	created, err := scanTag(s.executor.QueryRowContext(ctx, `
INSERT INTO tags (title, created_at, updated_at)
SELECT ?, ?, ?
WHERE NOT EXISTS (SELECT 1 FROM tags WHERE title = ? COLLATE NOCASE)
RETURNING `+tagColumns, name, timestamp, timestamp, name))
	if err == nil {
		return created, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return tag.Tag{}, fmt.Errorf("insert tag: %w", err)
	}

	existing, findErr := s.Find(ctx, name)
	if findErr != nil {
		return tag.Tag{}, fmt.Errorf("classify tag insert: %w", errors.Join(err, findErr))
	}
	return tag.Tag{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("tag already exists: %s", existing.Title),
		err,
	)
}

// Find returns the tag whose title matches name case-insensitively.
func (s *Tags) Find(ctx context.Context, name string) (tag.Tag, error) {
	return s.poolCore().Find(ctx, name)
}

func (s *tagsCore) Find(ctx context.Context, name string) (tag.Tag, error) {
	return findStoredTag(ctx, s.executor, name)
}

// List returns every tag with its usage count, ordered by title.
func (s *Tags) List(ctx context.Context) ([]tag.ListedTag, error) {
	return s.poolCore().List(ctx)
}

func (s *tagsCore) List(ctx context.Context) ([]tag.ListedTag, error) {
	rows, err := s.executor.QueryContext(ctx, `
SELECT `+tagColumns+`,
       `+tagUsageCountExpression+` AS usage_count
FROM tags
ORDER BY title COLLATE NOCASE, id
`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	return collectRows(rows, scanListedTag, "scan listed tag", "iterate listed tags")
}

// Rename retitles the tag matching oldName; a newName held by a different
// tag is a conflict error.
func (s *Tags) Rename(
	ctx context.Context,
	oldName, newName, timestamp string,
) (tag.Tag, error) {
	return runInTransaction(ctx, s.WithinTransaction, func(transaction tag.Transaction) (tag.Tag, error) {
		return transaction.Rename(ctx, oldName, newName, timestamp)
	})
}

func (s *tagsCore) Rename(
	ctx context.Context,
	oldName, newName, timestamp string,
) (tag.Tag, error) {
	renamed, err := scanTag(s.executor.QueryRowContext(ctx, `
UPDATE tags
SET title = ?, updated_at = ?
WHERE title = ? COLLATE NOCASE
  AND NOT EXISTS (
      SELECT 1
      FROM tags AS existing
      WHERE existing.title = ? COLLATE NOCASE
        AND existing.id != tags.id
  )
RETURNING `+tagColumns, newName, timestamp, oldName, newName))
	if err == nil {
		return renamed, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return tag.Tag{}, fmt.Errorf("rename tag: %w", err)
	}

	if _, findErr := s.Find(ctx, oldName); findErr != nil {
		return tag.Tag{}, findErr
	}
	existing, findErr := s.Find(ctx, newName)
	if findErr != nil {
		return tag.Tag{}, fmt.Errorf("classify tag rename: %w", errors.Join(err, findErr))
	}
	return tag.Tag{}, apperr.New(
		apperr.Conflict,
		fmt.Sprintf("tag already exists: %s", existing.Title),
		err,
	)
}

// CountUsage counts the tasks, projects, and areas the tag is attached to.
func (s *Tags) CountUsage(ctx context.Context, name string) (int64, error) {
	return s.poolCore().CountUsage(ctx, name)
}

func (s *tagsCore) CountUsage(ctx context.Context, name string) (int64, error) {
	var count int64
	err := s.executor.QueryRowContext(ctx, `
SELECT `+tagUsageCountExpression+`
FROM tags
WHERE title = ? COLLATE NOCASE
`, name).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		_, findErr := s.Find(ctx, name)
		return 0, findErr
	}
	if err != nil {
		return 0, fmt.Errorf("count tag usage: %w", err)
	}

	return count, nil
}

// Delete removes the tag; the schema detaches it from every row.
func (s *Tags) Delete(ctx context.Context, name string) (tag.Tag, error) {
	return s.poolCore().Delete(ctx, name)
}

func (s *tagsCore) Delete(ctx context.Context, name string) (tag.Tag, error) {
	deleted, err := scanTag(s.executor.QueryRowContext(
		ctx,
		"DELETE FROM tags WHERE title = ? COLLATE NOCASE RETURNING "+tagColumns,
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return tag.Tag{}, apperr.New(apperr.NotFound, fmt.Sprintf("no tag %s", name), err)
	}
	if err != nil {
		return tag.Tag{}, fmt.Errorf("delete tag: %w", err)
	}

	return deleted, nil
}

// WithinTransaction runs apply inside one BEGIN IMMEDIATE transaction and
// rolls back when it returns an error.
func (s *Tags) WithinTransaction(
	ctx context.Context,
	apply func(tag.Transaction) error,
) error {
	return withinImmediateTransaction(ctx, s.database, "tag", func(connection *sql.Conn) error {
		return apply(&tagsCore{executor: connection})
	})
}

func (s *Tags) poolCore() *tagsCore {
	return &tagsCore{executor: s.database.database}
}

func scanListedTag(scanner rowScanner) (tag.ListedTag, error) {
	var value tag.ListedTag
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.CreatedAt,
		&value.UpdatedAt,
		&value.UsageCount,
	)

	return value, err
}

func scanTag(scanner rowScanner) (tag.Tag, error) {
	var value tag.Tag
	err := scanner.Scan(
		&value.ID,
		&value.Title,
		&value.CreatedAt,
		&value.UpdatedAt,
	)

	return value, err
}
