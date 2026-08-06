package store

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/search"
)

func TestSearchIndexesOwnTextAcrossKindsAndHydratesCanonicalRowsAndContext(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO areas (id, title, note, archived_at, position, created_at, updated_at)
VALUES (1, 'Home', 'active area memo', NULL, 0, '2026-01-01T00:00:00.000Z', '2026-01-02T00:00:00.000Z'),
       (2, 'Cabin', 'lake retreat', '2026-02-01T00:00:00.000Z', 1, '2026-01-03T00:00:00.000Z', '2026-02-01T00:00:00.000Z');
INSERT INTO projects (id, area_id, title, note, done_at, position, created_at, updated_at)
VALUES (1, 1, 'Bathroom plumbing', 'project memo phrase', '2026-02-02T00:00:00.000Z', 0, '2026-01-04T00:00:00.000Z', '2026-02-02T00:00:00.000Z');
INSERT INTO tasks (id, project_id, title, note, position, created_at, updated_at)
VALUES (1, 1, 'Fix sink', 'member note', 0, '2026-01-05T00:00:00.000Z', '2026-01-05T00:00:00.000Z');
INSERT INTO tasks (id, area_id, title, note, position, created_at, updated_at)
VALUES (2, 1, 'Buy wrench', 'bathroom supply note', 0, '2026-01-06T00:00:00.000Z', '2026-01-06T00:00:00.000Z');
INSERT INTO tasks (id, title, note, done_at, position, created_at, updated_at)
VALUES (3, 'Call plumber', 'inbox note', '2026-02-03T00:00:00.000Z', 0, '2026-01-07T00:00:00.000Z', '2026-02-03T00:00:00.000Z');
INSERT INTO tags (id, title) VALUES (1, 'house'), (2, 'reno'), (3, 'errands');
INSERT INTO area_tags (area_id, tag_id) VALUES (1, 1), (2, 2);
INSERT INTO project_tags (project_id, tag_id) VALUES (1, 2);
INSERT INTO task_tags (task_id, tag_id) VALUES (3, 3);
`); err != nil {
		t.Fatalf("seed search documents: %v", err)
	}

	store := NewSearch(storage)
	assertSearchIdentitySet(t, store, ctx, "plumb*", false, []searchIdentity{
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 3},
	})
	assertSearchIdentitySet(t, store, ctx, `"project memo"`, false, []searchIdentity{{kind: search.KindProject, id: 1}})
	assertSearchIdentitySet(t, store, ctx, "errand*", false, []searchIdentity{{kind: search.KindTask, id: 3}})
	assertSearchIdentitySet(t, store, ctx, "retreat OR supply", false, []searchIdentity{
		{kind: search.KindTask, id: 2},
		{kind: search.KindArea, id: 2},
	})

	hits, err := store.Search(ctx, "reno", false)
	if err != nil {
		t.Fatalf("Search(reno) error = %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search(reno) = %#v, want project 1 and area 2 own-tag hits", hits)
	}
	var projectHit, areaHit search.Hit
	for _, hit := range hits {
		switch hit.Kind {
		case search.KindProject:
			projectHit = hit
		case search.KindArea:
			areaHit = hit
		}
	}

	if projectHit.Project == nil || projectHit.Project.ID != 1 || projectHit.Project.Status != "done" ||
		!reflect.DeepEqual(projectHit.Project.Tags, domain.TagNames{"reno"}) ||
		projectHit.GoverningAreaTitle == nil || *projectHit.GoverningAreaTitle != "Home" {
		t.Errorf("hydrated project hit = %#v, want complete resolved row, tags, and Home context", projectHit)
	}
	if areaHit.Area == nil || areaHit.Area.ID != 2 || areaHit.Area.ArchivedAt == nil ||
		!reflect.DeepEqual(areaHit.Area.Tags, domain.TagNames{"reno"}) {
		t.Errorf("hydrated area hit = %#v, want complete archived row and tags", areaHit)
	}

	taskHits, err := store.Search(ctx, "sink", false)
	if err != nil {
		t.Fatalf("Search(sink) error = %v", err)
	}
	if len(taskHits) != 1 || taskHits[0].Task == nil || taskHits[0].Task.ID != 1 ||
		taskHits[0].ProjectTitle == nil || *taskHits[0].ProjectTitle != "Bathroom plumbing" ||
		taskHits[0].GoverningAreaTitle == nil || *taskHits[0].GoverningAreaTitle != "Home" ||
		taskHits[0].Task.Tags == nil {
		t.Errorf("hydrated task hit = %#v, want complete row and both context titles", taskHits)
	}

	assertSearchIdentitySet(t, store, ctx, "house", false, []searchIdentity{{kind: search.KindArea, id: 1}})
}

func TestSearchDocumentAssemblyIncludesInheritedTitlesAndTags(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO areas (id, title, position) VALUES (1, 'Home', 0);
INSERT INTO projects (id, area_id, title, position) VALUES (1, 1, 'Bathroom plumbing', 0);
INSERT INTO tasks (id, project_id, title, position) VALUES (1, 1, 'Project task', 0);
INSERT INTO tasks (id, area_id, title, position) VALUES (2, 1, 'Area task', 0);
INSERT INTO tasks (id, title, position) VALUES (3, 'Inbox task', 0);
INSERT INTO tags (id, title) VALUES (1, 'house'), (2, 'reno'), (3, 'alpha');
INSERT INTO area_tags (area_id, tag_id) VALUES (1, 1);
INSERT INTO project_tags (project_id, tag_id) VALUES (1, 2), (1, 3);
`); err != nil {
		t.Fatalf("seed inherited search context: %v", err)
	}

	got := make(map[searchIdentity]string)
	err := withinDeferredTransaction(ctx, storage, "search assembly test", func(connection *sql.Conn) error {
		if _, createErr := connection.ExecContext(ctx, createSearchIndex); createErr != nil {
			return createErr
		}
		defer func() { _, _ = connection.ExecContext(context.WithoutCancel(ctx), "DROP TABLE temp.search_index") }()
		if _, populateErr := connection.ExecContext(ctx, populateSearchIndex); populateErr != nil {
			return populateErr
		}
		rows, queryErr := connection.QueryContext(ctx, "SELECT kind, entity_id, context FROM search_index")
		if queryErr != nil {
			return queryErr
		}
		assembled, collectErr := collectRows(rows, func(scanner rowScanner) (struct {
			identity searchIdentity
			context  string
		}, error) {
			var row struct {
				identity searchIdentity
				context  string
			}
			scanErr := scanner.Scan(&row.identity.kind, &row.identity.id, &row.context)
			return row, scanErr
		}, "scan assembled search document", "iterate assembled search documents")
		if collectErr != nil {
			return collectErr
		}
		for _, row := range assembled {
			got[row.identity] = row.context
		}
		return nil
	})
	if err != nil {
		t.Fatalf("assemble search documents: %v", err)
	}

	want := map[searchIdentity]string{
		{kind: search.KindTask, id: 1}:    "Bathroom plumbing alpha reno Home house",
		{kind: search.KindTask, id: 2}:    "Home house",
		{kind: search.KindTask, id: 3}:    "",
		{kind: search.KindProject, id: 1}: "Home house",
		{kind: search.KindArea, id: 1}:    "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("assembled contexts = %#v, want %#v", got, want)
	}

	assertSearchIdentities(t, NewSearch(storage), ctx, `"alpha reno"`, false, []searchIdentity{
		{kind: search.KindProject, id: 1},
	})
}

func TestSearchRelatedPullsMembersFromContainerTitlesAndTagsWhileDirectDoesNot(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO areas (id, title, position) VALUES (1, 'Homestead', 0);
INSERT INTO projects (id, area_id, title, position) VALUES (1, 1, 'Bathroom plumbing', 0);
INSERT INTO tasks (id, project_id, title, position) VALUES (1, 1, 'Fix sink', 0);
INSERT INTO tags (id, title) VALUES (1, 'dwelling'), (2, 'renovation');
INSERT INTO area_tags (area_id, tag_id) VALUES (1, 1);
INSERT INTO project_tags (project_id, tag_id) VALUES (1, 2);
`); err != nil {
		t.Fatalf("seed related search contexts: %v", err)
	}

	store := NewSearch(storage)
	for _, test := range []struct {
		expression  string
		directWant  []searchIdentity
		relatedWant []searchIdentity
	}{
		{
			expression:  "plumbing",
			directWant:  []searchIdentity{{kind: search.KindProject, id: 1}},
			relatedWant: []searchIdentity{{kind: search.KindProject, id: 1}, {kind: search.KindTask, id: 1}},
		},
		{
			expression:  "renovation",
			directWant:  []searchIdentity{{kind: search.KindProject, id: 1}},
			relatedWant: []searchIdentity{{kind: search.KindProject, id: 1}, {kind: search.KindTask, id: 1}},
		},
		{
			expression:  "homestead",
			directWant:  []searchIdentity{{kind: search.KindArea, id: 1}},
			relatedWant: []searchIdentity{{kind: search.KindArea, id: 1}, {kind: search.KindProject, id: 1}, {kind: search.KindTask, id: 1}},
		},
		{
			expression:  "dwelling",
			directWant:  []searchIdentity{{kind: search.KindArea, id: 1}},
			relatedWant: []searchIdentity{{kind: search.KindArea, id: 1}, {kind: search.KindProject, id: 1}, {kind: search.KindTask, id: 1}},
		},
	} {
		t.Run(test.expression, func(t *testing.T) {
			assertSearchIdentitySet(t, store, ctx, test.expression, false, test.directWant)
			assertSearchIdentitySet(t, store, ctx, test.expression, true, test.relatedWant)
		})
	}
}

func TestSearchRelatedOrdersContextOnlyHitsBelowDirectHits(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO projects (id, title, position) VALUES (1, 'signal', 0);
INSERT INTO tasks (id, project_id, title, position) VALUES (1, 1, 'neutral', 0);
`); err != nil {
		t.Fatalf("seed related search ranking: %v", err)
	}

	assertSearchIdentities(t, NewSearch(storage), ctx, "signal", true, []searchIdentity{
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 1},
	})
}

func TestSearchRelatedReflectsContainerAndTagRenamesImmediately(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO areas (id, title, position) VALUES (1, 'Home', 0);
INSERT INTO projects (id, area_id, title, position) VALUES (1, 1, 'Legacy plumbing', 0);
INSERT INTO tasks (id, project_id, title, position) VALUES (1, 1, 'Fix sink', 0);
INSERT INTO tags (id, title) VALUES (1, 'legacytopic');
INSERT INTO area_tags (area_id, tag_id) VALUES (1, 1);
`); err != nil {
		t.Fatalf("seed rename freshness contexts: %v", err)
	}

	store := NewSearch(storage)
	assertSearchIdentitySet(t, store, ctx, "legacy", true, []searchIdentity{
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 1},
	})
	if _, err := storage.database.ExecContext(ctx, "UPDATE projects SET title = 'Current remodel' WHERE id = 1"); err != nil {
		t.Fatalf("rename project: %v", err)
	}
	assertSearchIdentities(t, store, ctx, "legacy", true, []searchIdentity{})
	assertSearchIdentitySet(t, store, ctx, "remodel", true, []searchIdentity{
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 1},
	})

	assertSearchIdentitySet(t, store, ctx, "legacytopic", true, []searchIdentity{
		{kind: search.KindArea, id: 1},
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 1},
	})
	if _, err := storage.database.ExecContext(ctx, "UPDATE tags SET title = 'currenttopic' WHERE id = 1"); err != nil {
		t.Fatalf("rename tag: %v", err)
	}
	assertSearchIdentities(t, store, ctx, "legacytopic", true, []searchIdentity{})
	assertSearchIdentitySet(t, store, ctx, "currenttopic", true, []searchIdentity{
		{kind: search.KindArea, id: 1},
		{kind: search.KindProject, id: 1},
		{kind: search.KindTask, id: 1},
	})
}

func TestSearchOrdersByWeightedRelevanceThenKindAndID(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO tasks (id, title, note, position) VALUES
    (1, 'signal', '', 0),
    (2, 'neutral', '', 1),
    (3, 'neutral', 'signal', 2),
    (10, 'equal', '', 3),
    (4, 'equal', '', 4);
INSERT INTO projects (id, title, note, position) VALUES (8, 'equal', '', 0);
INSERT INTO areas (id, title, note, position) VALUES (7, 'equal', '', 0);
INSERT INTO tags (id, title) VALUES (1, 'signal');
INSERT INTO task_tags (task_id, tag_id) VALUES (2, 1);
`); err != nil {
		t.Fatalf("seed ranked search documents: %v", err)
	}

	store := NewSearch(storage)
	assertSearchIdentities(t, store, ctx, "signal", false, []searchIdentity{
		{kind: search.KindTask, id: 1},
		{kind: search.KindTask, id: 2},
		{kind: search.KindTask, id: 3},
	})
	assertSearchIdentities(t, store, ctx, "equal", false, []searchIdentity{
		{kind: search.KindTask, id: 4},
		{kind: search.KindTask, id: 10},
		{kind: search.KindProject, id: 8},
		{kind: search.KindArea, id: 7},
	})
}

func TestSearchHydratesAcrossBatches(t *testing.T) {
	ctx, storage := openTestStorage(t)
	const matchCount = 1001
	if _, err := storage.database.ExecContext(ctx, `
WITH RECURSIVE ids(id) AS (
    VALUES (1)
    UNION ALL
    SELECT id + 1 FROM ids WHERE id < ?
)
INSERT INTO tasks (id, title, position)
SELECT id, 'batchtoken', id - 1 FROM ids
`, matchCount); err != nil {
		t.Fatalf("seed batched search documents: %v", err)
	}

	hits, err := NewSearch(storage).Search(ctx, "batchtoken", false)
	if err != nil {
		t.Fatalf("Search(batchtoken) error = %v", err)
	}
	if len(hits) != matchCount {
		t.Fatalf("Search(batchtoken) hit count = %d, want %d", len(hits), matchCount)
	}
	if hits[0].Task == nil || hits[0].Task.ID != 1 ||
		hits[len(hits)-1].Task == nil || hits[len(hits)-1].Task.ID != matchCount {
		t.Errorf("Search(batchtoken) bounds = %#v/%#v, want task IDs 1/%d", hits[0], hits[len(hits)-1], matchCount)
	}
}

func TestSearchMapsMalformedFTSExpressionAndCleansTemporaryIndex(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	if _, err := storage.database.ExecContext(ctx, "INSERT INTO tasks (title, position) VALUES ('plumber', 0)"); err != nil {
		t.Fatalf("seed searchable task: %v", err)
	}
	store := NewSearch(storage)

	for _, test := range []struct {
		expression string
		detail     string
	}{
		{expression: "plumb* AND", detail: "fts5: syntax error"},
		{expression: `"unterminated`, detail: "unterminated string"},
		{expression: "in:home", detail: "no such column: in"},
	} {
		_, err := store.Search(ctx, test.expression, false)
		if code, _ := apperr.CodeOf(err); code != apperr.InvalidArgument {
			t.Errorf("Search(%q) error = %v, want invalid_argument", test.expression, err)
		}
		if err == nil || !strings.Contains(err.Error(), test.detail) {
			t.Errorf("Search(%q) error = %q, want parser detail %q", test.expression, err, test.detail)
		}
		assertNoTemporarySearchIndex(t, storage)
	}

	for attempt := range 2 {
		hits, searchErr := store.Search(ctx, "plumb*", false)
		if searchErr != nil {
			t.Fatalf("Search(valid attempt %d) error = %v", attempt, searchErr)
		}
		if len(hits) != 1 || hits[0].Task == nil || hits[0].Task.Title != "plumber" {
			t.Errorf("Search(valid attempt %d) = %#v, want plumber", attempt, hits)
		}
		assertNoTemporarySearchIndex(t, storage)
	}
}

func TestSearchEmptyResultIsNonNilAndLeavesNoPersistentSchema(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	hits, err := NewSearch(storage).Search(ctx, "absent", false)
	if err != nil {
		t.Fatalf("Search(absent) error = %v", err)
	}
	if hits == nil || len(hits) != 0 {
		t.Errorf("Search(absent) = %#v, want nonnil empty slice", hits)
	}

	var version int
	if err := storage.database.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaRevision {
		t.Errorf("user_version = %d, want unchanged %d", version, schemaRevision)
	}
	assertNoTemporarySearchIndex(t, storage)
}

func assertSearchIdentitySet(
	t *testing.T,
	store *Search,
	ctx context.Context,
	expression string,
	related bool,
	want []searchIdentity,
) {
	t.Helper()
	hits, err := store.Search(ctx, expression, related)
	if err != nil {
		t.Fatalf("Search(%q) error = %v", expression, err)
	}
	remaining := make(map[searchIdentity]int, len(want))
	for _, identity := range want {
		remaining[identity]++
	}
	for _, hit := range hits {
		identity := searchIdentity{kind: hit.Kind}
		switch hit.Kind {
		case search.KindTask:
			identity.id = hit.Task.ID
		case search.KindProject:
			identity.id = hit.Project.ID
		case search.KindArea:
			identity.id = hit.Area.ID
		}
		remaining[identity]--
	}
	for _, count := range remaining {
		if count != 0 {
			t.Errorf("Search(%q) = %#v, want identity set %#v", expression, hits, want)
			return
		}
	}
	if len(hits) != len(want) {
		t.Errorf("Search(%q) = %#v, want identity set %#v", expression, hits, want)
	}
}

func assertSearchIdentities(
	t *testing.T,
	store *Search,
	ctx context.Context,
	expression string,
	related bool,
	want []searchIdentity,
) {
	t.Helper()
	hits, err := store.Search(ctx, expression, related)
	if err != nil {
		t.Fatalf("Search(%q) error = %v", expression, err)
	}
	got := make([]searchIdentity, len(hits))
	for index, hit := range hits {
		got[index].kind = hit.Kind
		switch hit.Kind {
		case search.KindTask:
			got[index].id = hit.Task.ID
		case search.KindProject:
			got[index].id = hit.Project.ID
		case search.KindArea:
			got[index].id = hit.Area.ID
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Search(%q) identities = %#v, want %#v", expression, got, want)
	}
}

func assertNoTemporarySearchIndex(t *testing.T, storage *DB) {
	t.Helper()
	var count int
	if err := storage.database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM sqlite_temp_master WHERE name = 'search_index'",
	).Scan(&count); err != nil {
		t.Fatalf("inspect temp search index: %v", err)
	}
	if count != 0 {
		t.Errorf("temporary search_index count = %d, want 0", count)
	}
}
