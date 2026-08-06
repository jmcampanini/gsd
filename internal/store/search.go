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
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/task"
	modernsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const searchHydrationBatchSize = 1000

const createSearchIndex = `
CREATE VIRTUAL TABLE temp.search_index USING fts5(
    kind UNINDEXED,
    entity_id UNINDEXED,
    title,
    tags,
    note,
    context
)`

const populateSearchIndex = `
WITH task_tag_text AS (
    SELECT et.task_id AS entity_id,
           group_concat(g.title, ' ' ORDER BY g.title COLLATE NOCASE) AS titles
    FROM task_tags AS et
    JOIN tags AS g ON g.id = et.tag_id
    GROUP BY et.task_id
),
project_tag_text AS (
    SELECT et.project_id AS entity_id,
           group_concat(g.title, ' ' ORDER BY g.title COLLATE NOCASE) AS titles
    FROM project_tags AS et
    JOIN tags AS g ON g.id = et.tag_id
    GROUP BY et.project_id
),
area_tag_text AS (
    SELECT et.area_id AS entity_id,
           group_concat(g.title, ' ' ORDER BY g.title COLLATE NOCASE) AS titles
    FROM area_tags AS et
    JOIN tags AS g ON g.id = et.tag_id
    GROUP BY et.area_id
)
INSERT INTO search_index (kind, entity_id, title, tags, note, context)
SELECT 'task', t.id, t.title, COALESCE(tt.titles, ''), t.note,
       trim(COALESCE(p.title, '') || ' ' || COALESCE(pt.titles, '') || ' ' ||
            COALESCE(a.title, '') || ' ' || COALESCE(at.titles, ''))
FROM tasks AS t
LEFT JOIN task_tag_text AS tt ON tt.entity_id = t.id
LEFT JOIN projects AS p ON p.id = t.project_id
LEFT JOIN project_tag_text AS pt ON pt.entity_id = p.id
LEFT JOIN areas AS a ON a.id = COALESCE(t.area_id, p.area_id)
LEFT JOIN area_tag_text AS at ON at.entity_id = a.id
UNION ALL
SELECT 'project', p.id, p.title, COALESCE(pt.titles, ''), p.note,
       trim(COALESCE(a.title, '') || ' ' || COALESCE(at.titles, ''))
FROM projects AS p
LEFT JOIN project_tag_text AS pt ON pt.entity_id = p.id
LEFT JOIN areas AS a ON a.id = p.area_id
LEFT JOIN area_tag_text AS at ON at.entity_id = a.id
UNION ALL
SELECT 'area', a.id, a.title, COALESCE(at.titles, ''), a.note, ''
FROM areas AS a
LEFT JOIN area_tag_text AS at ON at.entity_id = a.id
`

type Search struct {
	database *DB
}

type searchIdentity struct {
	kind string
	id   int64
}

type taskSearchRow struct {
	value              task.Task
	projectTitle       *string
	governingAreaTitle *string
}

type projectSearchRow struct {
	value              project.Project
	governingAreaTitle *string
}

func NewSearch(database *DB) *Search {
	return &Search{database: database}
}

func (s *Search) Search(ctx context.Context, expression string, related bool) ([]search.Hit, error) {
	var hits []search.Hit
	err := withinDeferredTransaction(ctx, s.database, "search", func(connection *sql.Conn) (operationErr error) {
		if _, err := connection.ExecContext(ctx, createSearchIndex); err != nil {
			return fmt.Errorf("create search index: %w", err)
		}
		defer func() {
			_, dropErr := connection.ExecContext(context.WithoutCancel(ctx), "DROP TABLE temp.search_index")
			if dropErr != nil {
				operationErr = errors.Join(operationErr, fmt.Errorf("drop search index: %w", dropErr))
			}
		}()

		if _, err := connection.ExecContext(ctx, populateSearchIndex); err != nil {
			return fmt.Errorf("populate search index: %w", err)
		}

		identities, err := matchSearchIndex(ctx, connection, expression, related)
		if err != nil {
			return mapSearchMatchError(err)
		}
		hits, err = hydrateSearchHits(ctx, connection, identities)
		return err
	})
	if err != nil {
		return nil, err
	}
	return hits, nil
}

func matchSearchIndex(
	ctx context.Context,
	connection *sql.Conn,
	expression string,
	related bool,
) ([]searchIdentity, error) {
	matchExpression := expression
	if !related {
		matchExpression = "{title tags note} : (" + expression + ")"
	}
	rows, err := connection.QueryContext(ctx, `
SELECT kind, entity_id
FROM search_index
WHERE search_index MATCH ?
ORDER BY bm25(search_index, 0.0, 0.0, 4.0, 3.0, 2.0, 1.0),
         CASE kind WHEN 'task' THEN 0 WHEN 'project' THEN 1 ELSE 2 END,
         CAST(entity_id AS INTEGER)
`, matchExpression)
	if err != nil {
		return nil, err
	}
	return collectRows(rows, func(scanner rowScanner) (searchIdentity, error) {
		var identity searchIdentity
		err := scanner.Scan(&identity.kind, &identity.id)
		return identity, err
	}, "scan search match", "iterate search matches")
}

func mapSearchMatchError(err error) error {
	var sqliteError *modernsqlite.Error
	if errors.As(err, &sqliteError) && sqliteError.Code()&0xff == sqlite3.SQLITE_ERROR {
		return apperr.New(
			apperr.InvalidArgument,
			"invalid search expression: "+sqliteError.Error(),
			err,
		)
	}
	return fmt.Errorf("match search index: %w", err)
}

func hydrateSearchHits(
	ctx context.Context,
	connection *sql.Conn,
	identities []searchIdentity,
) ([]search.Hit, error) {
	idsByKind := map[string][]int64{
		search.KindTask:    {},
		search.KindProject: {},
		search.KindArea:    {},
	}
	for _, identity := range identities {
		idsByKind[identity.kind] = append(idsByKind[identity.kind], identity.id)
	}

	tasks, err := hydrateSearchTasks(ctx, connection, idsByKind[search.KindTask])
	if err != nil {
		return nil, err
	}
	projects, err := hydrateSearchProjects(ctx, connection, idsByKind[search.KindProject])
	if err != nil {
		return nil, err
	}
	areas, err := hydrateSearchAreas(ctx, connection, idsByKind[search.KindArea])
	if err != nil {
		return nil, err
	}

	hits := make([]search.Hit, 0, len(identities))
	for _, identity := range identities {
		switch identity.kind {
		case search.KindTask:
			row, ok := tasks[identity.id]
			if !ok {
				return nil, fmt.Errorf("hydrate search task %d: matched row disappeared", identity.id)
			}
			value := row.value
			hits = append(hits, search.Hit{
				Kind:               identity.kind,
				Task:               &value,
				ProjectTitle:       row.projectTitle,
				GoverningAreaTitle: row.governingAreaTitle,
			})
		case search.KindProject:
			row, ok := projects[identity.id]
			if !ok {
				return nil, fmt.Errorf("hydrate search project %d: matched row disappeared", identity.id)
			}
			value := row.value
			hits = append(hits, search.Hit{
				Kind:               identity.kind,
				Project:            &value,
				GoverningAreaTitle: row.governingAreaTitle,
			})
		case search.KindArea:
			value, ok := areas[identity.id]
			if !ok {
				return nil, fmt.Errorf("hydrate search area %d: matched row disappeared", identity.id)
			}
			hits = append(hits, search.Hit{Kind: identity.kind, Area: &value})
		default:
			return nil, fmt.Errorf("hydrate search match: unknown kind %q", identity.kind)
		}
	}
	return hits, nil
}

func hydrateSearchTasks(
	ctx context.Context,
	connection *sql.Conn,
	ids []int64,
) (map[int64]taskSearchRow, error) {
	values := make(map[int64]taskSearchRow, len(ids))
	for start := 0; start < len(ids); start += searchHydrationBatchSize {
		batch := ids[start:min(start+searchHydrationBatchSize, len(ids))]
		rows, err := connection.QueryContext(ctx, `
SELECT `+qualifiedColumns("matched", taskColumns)+`,
       `+tagJSONExpression(taskTagSpec, "matched.id")+` AS tags,
       p.title,
       a.title
FROM tasks AS matched
LEFT JOIN projects AS p ON p.id = matched.project_id
LEFT JOIN areas AS a ON a.id = COALESCE(matched.area_id, p.area_id)
WHERE matched.id IN (`+queryPlaceholders(len(batch))+`)
`, int64Arguments(batch)...)
		if err != nil {
			return nil, fmt.Errorf("query matched tasks: %w", err)
		}
		collected, err := collectRows(rows, func(scanner rowScanner) (taskSearchRow, error) {
			var row taskSearchRow
			targets := append(taskBaseScanTargets(&row.value), scanTagTitles(&row.value.Tags))
			targets = append(targets, &row.projectTitle, &row.governingAreaTitle)
			err := scanner.Scan(targets...)
			return row, err
		}, "scan matched task", "iterate matched tasks")
		if err != nil {
			return nil, err
		}
		for _, row := range collected {
			values[row.value.ID] = row
		}
	}
	return values, nil
}

func hydrateSearchProjects(
	ctx context.Context,
	connection *sql.Conn,
	ids []int64,
) (map[int64]projectSearchRow, error) {
	values := make(map[int64]projectSearchRow, len(ids))
	for start := 0; start < len(ids); start += searchHydrationBatchSize {
		batch := ids[start:min(start+searchHydrationBatchSize, len(ids))]
		rows, err := connection.QueryContext(ctx, `
SELECT `+qualifiedColumns("matched", projectColumns)+`,
       `+tagJSONExpression(projectTagSpec, "matched.id")+` AS tags,
       a.title
FROM projects AS matched
LEFT JOIN areas AS a ON a.id = matched.area_id
WHERE matched.id IN (`+queryPlaceholders(len(batch))+`)
`, int64Arguments(batch)...)
		if err != nil {
			return nil, fmt.Errorf("query matched projects: %w", err)
		}
		collected, err := collectRows(rows, func(scanner rowScanner) (projectSearchRow, error) {
			var row projectSearchRow
			targets := append(projectBaseScanTargets(&row.value), scanTagTitles(&row.value.Tags))
			targets = append(targets, &row.governingAreaTitle)
			err := scanner.Scan(targets...)
			return row, err
		}, "scan matched project", "iterate matched projects")
		if err != nil {
			return nil, err
		}
		for _, row := range collected {
			values[row.value.ID] = row
		}
	}
	return values, nil
}

func hydrateSearchAreas(
	ctx context.Context,
	connection *sql.Conn,
	ids []int64,
) (map[int64]area.Area, error) {
	values := make(map[int64]area.Area, len(ids))
	for start := 0; start < len(ids); start += searchHydrationBatchSize {
		batch := ids[start:min(start+searchHydrationBatchSize, len(ids))]
		rows, err := connection.QueryContext(ctx, `
SELECT `+qualifiedColumns("matched", areaColumns)+`,
       `+tagJSONExpression(areaTagSpec, "matched.id")+` AS tags
FROM areas AS matched
WHERE matched.id IN (`+queryPlaceholders(len(batch))+`)
`, int64Arguments(batch)...)
		if err != nil {
			return nil, fmt.Errorf("query matched areas: %w", err)
		}
		collected, err := collectRows(rows, scanArea, "scan matched area", "iterate matched areas")
		if err != nil {
			return nil, err
		}
		for _, value := range collected {
			values[value.ID] = value
		}
	}
	return values, nil
}

func qualifiedColumns(alias, columns string) string {
	return alias + "." + strings.ReplaceAll(columns, ", ", ", "+alias+".")
}

func queryPlaceholders(count int) string {
	return strings.TrimSuffix(strings.Repeat("?,", count), ",")
}

func int64Arguments(values []int64) []any {
	arguments := make([]any, len(values))
	for index, value := range values {
		arguments[index] = value
	}
	return arguments
}
