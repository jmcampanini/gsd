package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

func TestDirectSearchAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "search", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	for _, title := range []string{"house", "reno", "errands", "beacon"} {
		decodeTagRow(t, runJSON("tags", "add", title))
	}

	home := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	home = decodeAreaRow(t, runJSON("area", "tag", fmt.Sprint(home.ID), "house"))
	cabin := decodeAreaRow(t, runJSON("areas", "add", "Cabin", "--note", "lake retreat projects"))
	cabin = decodeAreaRow(t, runJSON("area", "tag", fmt.Sprint(cabin.ID), "house"))
	cabin = decodeAreaRow(t, runJSON("area", "archive", fmt.Sprint(cabin.ID)))
	decodeAreaRow(t, runJSON("areas", "add", "Beacon command"))

	bathroom := decodeProject(t, runJSON(
		"projects", "add", "Bathroom plumbing", "--area", fmt.Sprint(home.ID),
	))
	bathroom = decodeProject(t, runJSON("project", "tag", fmt.Sprint(bathroom.ID), "reno"))
	beaconProject := decodeProject(t, runJSON("projects", "add", "Reference guide"))
	beaconProject = decodeProject(t, runJSON(
		"project", "tag", fmt.Sprint(beaconProject.ID), "beacon",
	))
	legacy := decodeProject(t, runJSON("projects", "add", "Legacy migration"))
	legacy = decodeProjectResolution(t, runJSON(
		"project", "done", fmt.Sprint(legacy.ID),
	)).Project

	plumber := decodeTask(t, runJSON("add", "Call plumber"))
	plumber = decodeTask(t, runJSON("tag", fmt.Sprint(plumber.ID), "errands"))
	loose := decodeTask(t, runJSON(
		"add", "Buy pipe wrench", "--area", fmt.Sprint(home.ID), "--note", "for the bathroom",
	))
	fixSink := decodeTask(t, runJSON(
		"add", "Fix sink", "--project", fmt.Sprint(bathroom.ID),
	))
	tiles := decodeTask(t, runJSON(
		"add", "Order tiles", "--project", fmt.Sprint(bathroom.ID),
	))
	tiles = decodeTask(t, runJSON("done", fmt.Sprint(tiles.ID)))
	manual := decodeTask(t, runJSON(
		"add", "Read manual", "--note", "beacon staleonly",
	))

	if plumber.ProjectID != nil || plumber.AreaID != nil || loose.AreaID == nil ||
		loose.ProjectID != nil || fixSink.ProjectID == nil || tiles.Status != "done" || legacy.Status != "done" ||
		cabin.ArchivedAt == nil {
		t.Fatalf(
			"search seed = plumber %#v, loose %#v, project task %#v, resolved task %#v, resolved project %#v, archived area %#v",
			plumber, loose, fixSink, tiles, legacy, cabin,
		)
	}

	prefix := decodeSearchHits(t, runJSON("search", "plumb*"))
	assertSearchTitles(t, prefix, "Bathroom plumbing", "Call plumber")
	if searchHasID(prefix, "task", fixSink.ID) || searchHasID(prefix, "task", tiles.ID) {
		t.Errorf("direct prefix hits = %#v, want project members excluded when only their context matches", prefix)
	}

	phrase := decodeSearchHits(t, runJSON("search", `"Bathroom plumbing"`))
	assertSearchTitles(t, phrase, "Bathroom plumbing")

	orHits := decodeSearchHits(t, runJSON("search", "tile* OR errand*"))
	assertSearchTitles(t, orHits, "Call plumber", "Order tiles")

	directContext := decodeSearchHits(t, runJSON("search", "Home"))
	assertSearchTitles(t, directContext, "Home")

	ranked := decodeSearchHits(t, runJSON("search", "beacon"))
	if got, want := searchTitles(ranked), []string{"Beacon command", "Reference guide", "Read manual"}; !reflect.DeepEqual(got, want) {
		t.Errorf("beacon ranking = %#v, want title, tag, then note hits %#v", got, want)
	}

	included := decodeSearchHits(t, runJSON("search", "tile* OR legacy OR cabin"))
	assertSearchTitles(t, included, "Cabin", "Legacy migration", "Order tiles")
	assertSearchField(t, included, "Order tiles", "status", "done")
	assertSearchField(t, included, "Legacy migration", "status", "done")
	if hit := searchHitByTitle(t, included, "Cabin"); string(hit["archived_at"]) == "null" {
		t.Errorf("archived Cabin hit = %s, want non-null archived_at", hit["archived_at"])
	}

	canonical := decodeSearchHits(t, runJSON("search", "errand* OR reno OR lake"))
	if len(canonical) != 3 {
		t.Fatalf("canonical search hits = %#v, want one hit of each kind", canonical)
	}
	assertCanonicalSearchHit(t, searchHitByKind(t, canonical, "task"), "task", plumber)
	assertCanonicalSearchHit(t, searchHitByKind(t, canonical, "project"), "project", bathroom)
	assertCanonicalSearchHit(t, searchHitByKind(t, canonical, "area"), "area", cabin)

	manual = decodeTask(t, runJSON(
		"edit", fmt.Sprint(manual.ID), "--note", "beacon freshonly",
	))
	assertSearchTitles(t, decodeSearchHits(t, runJSON("search", "staleonly")))
	fresh := decodeSearchHits(t, runJSON("search", "freshonly"))
	assertSearchTitles(t, fresh, manual.Title)
	assertCanonicalSearchHit(t, fresh[0], "task", manual)

	human := runGSD(
		t,
		"search", `tile* OR legacy OR cabin OR "Bathroom plumbing"`,
		"--db", databasePath,
	)
	if human.exitCode != 0 || human.stderr != "" || human.stdout == "" ||
		strings.Contains(human.stdout, "\x1b[") {
		t.Fatalf("human search = %#v, want plain stdout-only table", human)
	}
	normalizedHuman := strings.ToLower(strings.Join(strings.Fields(human.stdout), " "))
	for _, fragment := range []string{
		"kind", "id", "title", "status", "context",
		"task", "order tiles", "done", "bathroom plumbing · home",
		"project", "legacy migration", "area", "cabin", "archived",
	} {
		if !strings.Contains(normalizedHuman, fragment) {
			t.Errorf("human search = %q, want fragment %q", normalizedHuman, fragment)
		}
	}

	emptyHuman := runGSD(t, "search", "definitelyabsenttoken", "--db", databasePath)
	if emptyHuman.exitCode != 0 || emptyHuman.stdout != "" || emptyHuman.stderr != "" {
		t.Errorf("empty human search = %#v, want no output", emptyHuman)
	}
	emptyJSON := runJSON("search", "definitelyabsenttoken")
	if emptyJSON.exitCode != 0 || emptyJSON.stderr != "" || emptyJSON.stdout != "[]\n" {
		t.Errorf("empty JSON search = %#v, want []", emptyJSON)
	}

	for description, result := range map[string]processResult{
		"blank expression":     runJSON("search", "   "),
		"malformed expression": runJSON("search", "plumb* AND"),
	} {
		assertJSONError(t, result, apperr.InvalidArgument)
		assertSearchDidNotPanic(t, description, result)
	}
}

func TestSearchSyntaxFailuresDoNotOpenDatabase(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing expression", args: []string{"search"}},
		{name: "extra expression", args: []string{"search", "one", "two"}},
		{name: "unknown flag", args: []string{"search", "one", "--unknown"}},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			databasePath := filepath.Join(workDir, "search-syntax", fmt.Sprint(index), "unused.db")
			args := append(append([]string{}, test.args...), "--db", databasePath, "--json")
			result := runGSD(t, args...)
			if result.exitCode != 2 || result.stdout != "" || result.stderr == "" {
				t.Errorf("syntax result = %#v, want stderr-only usage exit 2", result)
			}
			assertSearchDidNotPanic(t, test.name, result)
			if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("database stat error = %v, want database not created", err)
			}
		})
	}
}

type searchJSONHit map[string]json.RawMessage

func decodeSearchHits(t *testing.T, result processResult) []searchJSONHit {
	t.Helper()
	return decodeJSON[[]searchJSONHit](t, result, "search hits")
}

func assertSearchTitles(t *testing.T, hits []searchJSONHit, want ...string) {
	t.Helper()
	got := searchTitles(hits)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Errorf("search titles = %#v, want %#v", got, want)
		return
	}
	for index := range got {
		if got[index] != want[index] {
			t.Errorf("search titles = %#v, want %#v", got, want)
			return
		}
	}
}

func searchTitles(hits []searchJSONHit) []string {
	titles := make([]string, len(hits))
	for index, hit := range hits {
		_ = json.Unmarshal(hit["title"], &titles[index])
	}
	return titles
}

func searchHasID(hits []searchJSONHit, kind string, id int64) bool {
	for _, hit := range hits {
		var hitKind string
		var hitID int64
		_ = json.Unmarshal(hit["kind"], &hitKind)
		_ = json.Unmarshal(hit["id"], &hitID)
		if hitKind == kind && hitID == id {
			return true
		}
	}
	return false
}

func searchHitByTitle(t *testing.T, hits []searchJSONHit, title string) searchJSONHit {
	t.Helper()
	for _, hit := range hits {
		var current string
		if err := json.Unmarshal(hit["title"], &current); err != nil {
			t.Fatalf("decode search title: %v", err)
		}
		if current == title {
			return hit
		}
	}
	t.Fatalf("search hits = %#v, want title %q", hits, title)
	return nil
}

func searchHitByKind(t *testing.T, hits []searchJSONHit, kind string) searchJSONHit {
	t.Helper()
	for _, hit := range hits {
		var current string
		if err := json.Unmarshal(hit["kind"], &current); err != nil {
			t.Fatalf("decode search kind: %v", err)
		}
		if current == kind {
			return hit
		}
	}
	t.Fatalf("search hits = %#v, want kind %q", hits, kind)
	return nil
}

func assertSearchField(t *testing.T, hits []searchJSONHit, title, field, want string) {
	t.Helper()
	hit := searchHitByTitle(t, hits, title)
	var got string
	if err := json.Unmarshal(hit[field], &got); err != nil {
		t.Fatalf("decode %s for %q: %v", field, title, err)
	}
	if got != want {
		t.Errorf("%s for %q = %q, want %q", field, title, got, want)
	}
}

func assertCanonicalSearchHit(t *testing.T, got searchJSONHit, kind string, entity any) {
	t.Helper()
	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("marshal canonical %s: %v", kind, err)
	}
	var want searchJSONHit
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("decode canonical %s: %v", kind, err)
	}
	want["kind"] = json.RawMessage(fmt.Sprintf("%q", kind))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s search hit = %s, want canonical flattened row %s", kind, mustJSON(got), mustJSON(want))
	}
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func assertSearchDidNotPanic(t *testing.T, description string, result processResult) {
	t.Helper()
	if strings.Contains(strings.ToLower(result.stderr), "panic") {
		t.Errorf("%s stderr = %q, want no panic", description, result.stderr)
	}
}
