package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tag"
)

type entityTagSpec struct {
	noun         string
	joinTable    string
	entityColumn string
}

var (
	taskTagSpec    = entityTagSpec{noun: "task", joinTable: "task_tags", entityColumn: "task_id"}
	projectTagSpec = entityTagSpec{noun: "project", joinTable: "project_tags", entityColumn: "project_id"}
	areaTagSpec    = entityTagSpec{noun: "area", joinTable: "area_tags", entityColumn: "area_id"}
)

type tagQueryExecutor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type tagWriteExecutor interface {
	tagQueryExecutor
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func findStoredTag(ctx context.Context, executor tagQueryExecutor, name string) (tag.Tag, error) {
	found, err := scanTag(executor.QueryRowContext(
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

func resolveStoredTags(
	ctx context.Context,
	executor tagQueryExecutor,
	names []string,
) ([]tag.Tag, error) {
	resolved := make([]tag.Tag, 0, len(names))
	for _, name := range names {
		found, err := findStoredTag(ctx, executor, name)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, found)
	}
	return resolved, nil
}

func attachEntityTags(
	ctx context.Context,
	executor tagWriteExecutor,
	spec entityTagSpec,
	entityID int64,
	tags []tag.Tag,
) error {
	query := "INSERT OR IGNORE INTO " + spec.joinTable + " (" + spec.entityColumn + ", tag_id) VALUES (?, ?)"
	for _, current := range tags {
		if _, err := executor.ExecContext(ctx, query, entityID, current.ID); err != nil {
			return fmt.Errorf("attach %s tag: %w", spec.noun, err)
		}
	}
	return nil
}

func detachEntityTags(
	ctx context.Context,
	executor tagWriteExecutor,
	spec entityTagSpec,
	entityID int64,
	tags []tag.Tag,
) error {
	query := "DELETE FROM " + spec.joinTable + " WHERE " + spec.entityColumn + " = ? AND tag_id = ?"
	for _, current := range tags {
		if _, err := executor.ExecContext(ctx, query, entityID, current.ID); err != nil {
			return fmt.Errorf("detach %s tag: %w", spec.noun, err)
		}
	}
	return nil
}

func tagJSONExpression(spec entityTagSpec, entityReference string) string {
	return `(SELECT json_group_array(g.title ORDER BY g.title COLLATE NOCASE)
        FROM ` + spec.joinTable + ` et JOIN tags g ON g.id = et.tag_id
        WHERE et.` + spec.entityColumn + ` = ` + entityReference + `)`
}

type tagTitlesScanner[T ~[]string] struct {
	destination *T
}

func scanTagTitles[T ~[]string](destination *T) *tagTitlesScanner[T] {
	return &tagTitlesScanner[T]{destination: destination}
}

func (s *tagTitlesScanner[T]) Scan(source any) error {
	var encoded []byte
	switch value := source.(type) {
	case string:
		encoded = []byte(value)
	case []byte:
		encoded = value
	case nil:
		*s.destination = make(T, 0)
		return nil
	default:
		return fmt.Errorf("scan tag titles from %T", source)
	}

	var titles []string
	if err := json.Unmarshal(encoded, &titles); err != nil {
		return fmt.Errorf("decode tag titles: %w", err)
	}
	if titles == nil {
		titles = []string{}
	}
	*s.destination = T(titles)
	return nil
}
