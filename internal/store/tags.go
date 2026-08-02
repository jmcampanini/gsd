package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tag"
)

const tagColumns = `id, title, created_at, updated_at`

type Tags struct {
	database *DB
	executor tagExecutor
}

type tagExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func NewTags(database *DB) *Tags {
	return &Tags{database: database, executor: database.database}
}

func (s *Tags) Add(ctx context.Context, name, timestamp string) (tag.Tag, error) {
	if s.database != nil {
		return runInTransaction(ctx, s.WithinTransaction, func(transaction tag.Store) (tag.Tag, error) {
			return transaction.Add(ctx, name, timestamp)
		})
	}

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

func (s *Tags) Find(ctx context.Context, name string) (tag.Tag, error) {
	found, err := scanTag(s.executor.QueryRowContext(
		ctx,
		"SELECT "+tagColumns+" FROM tags WHERE title = ? COLLATE NOCASE",
		name,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return tag.Tag{}, apperr.New(apperr.NotFound, fmt.Sprintf("no tag %s", name), err)
	}
	if err != nil {
		return tag.Tag{}, fmt.Errorf("find tag: %w", err)
	}

	return found, nil
}

func (s *Tags) List(ctx context.Context) ([]tag.ListedTag, error) {
	rows, err := s.executor.QueryContext(ctx, `
SELECT `+tagColumns+`,
       (SELECT COUNT(*) FROM task_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM project_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM area_tags WHERE tag_id = tags.id) AS usage_count
FROM tags
ORDER BY title COLLATE NOCASE, id
`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer func() { _ = rows.Close() }()

	listed := make([]tag.ListedTag, 0)
	for rows.Next() {
		var current tag.ListedTag
		if err := rows.Scan(
			&current.ID,
			&current.Title,
			&current.CreatedAt,
			&current.UpdatedAt,
			&current.UsageCount,
		); err != nil {
			return nil, fmt.Errorf("scan listed tag: %w", err)
		}
		listed = append(listed, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed tags: %w", err)
	}

	return listed, nil
}

func (s *Tags) Rename(
	ctx context.Context,
	oldName, newName, timestamp string,
) (tag.Tag, error) {
	if s.database != nil {
		return runInTransaction(ctx, s.WithinTransaction, func(transaction tag.Store) (tag.Tag, error) {
			return transaction.Rename(ctx, oldName, newName, timestamp)
		})
	}

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

func (s *Tags) CountUsage(ctx context.Context, name string) (int64, error) {
	var count int64
	err := s.executor.QueryRowContext(ctx, `
SELECT (SELECT COUNT(*) FROM task_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM project_tags WHERE tag_id = tags.id) +
       (SELECT COUNT(*) FROM area_tags WHERE tag_id = tags.id)
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

func (s *Tags) Delete(ctx context.Context, name string) (tag.Tag, error) {
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

func (s *Tags) WithinTransaction(
	ctx context.Context,
	apply func(tag.Store) error,
) error {
	if s.database == nil {
		return errors.New("nested tag transactions are not supported")
	}

	return withinImmediateTransaction(ctx, s.database, "tag", func(connection *sql.Conn) error {
		return apply(&Tags{executor: connection})
	})
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
