package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeSearchApplication struct {
	result     []search.Hit
	err        error
	calls      int
	expression string
}

func (f *fakeSearchApplication) Search(_ context.Context, expression string) ([]search.Hit, error) {
	f.calls++
	f.expression = expression
	return f.result, f.err
}

func runSearchCommand(t *testing.T, application search.Application, args ...string) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{search: application}, strings.NewReader(""), args...)
}

func TestSearchJSONWritesFlattenedCanonicalRowsInResultOrder(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	areaID := int64(4)
	projectTitle := "Bathroom"
	areaTitle := "Home"
	archivedAt := "2026-08-04T12:00:00Z"
	taskRow := task.Task{
		ID: 3, ProjectID: &projectID, Title: "Fix <sink> & drain", Note: "note", Status: "done",
		Position: 2, CreatedAt: "2026-08-01T12:00:00Z", UpdatedAt: "2026-08-02T12:00:00Z",
		Tags: []string{"<repair> & upkeep"},
	}
	projectRow := project.Project{
		ID: 7, AreaID: &areaID, Title: "Bathroom", Note: "project note", Status: "open",
		Position: 1, CreatedAt: "2026-08-01T12:00:00Z", UpdatedAt: "2026-08-02T12:00:00Z",
		Tags: []string{},
	}
	areaRow := area.Area{
		ID: 4, Title: "Home", Note: "area note", ArchivedAt: &archivedAt, Position: 1,
		CreatedAt: "2026-08-01T12:00:00Z", UpdatedAt: "2026-08-04T12:00:00Z", Tags: []string{"house"},
	}
	hits := []search.Hit{
		{Kind: search.KindTask, Task: &taskRow, ProjectTitle: &projectTitle, GoverningAreaTitle: &areaTitle},
		{Kind: search.KindProject, Project: &projectRow, GoverningAreaTitle: &areaTitle},
		{Kind: search.KindArea, Area: &areaRow},
	}
	application := &fakeSearchApplication{result: hits}
	result := runSearchCommand(t, application, "--db", "chosen.db", "search", "sink OR home", "--json")

	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want JSON success", result)
	}
	if application.calls != 1 || application.expression != "sink OR home" {
		t.Errorf("Search() call = %d/%q, want one exact expression", application.calls, application.expression)
	}
	if result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want selected DB and one open/close", result)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}
	if !strings.Contains(result.stdout, `"title":"Fix <sink> & drain"`) ||
		!strings.Contains(result.stdout, `"tags":["<repair> & upkeep"]`) {
		t.Errorf("stdout = %q, want shared non-HTML-escaped JSON policy", result.stdout)
	}

	var objects []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &objects); err != nil {
		t.Fatalf("decode search JSON: %v", err)
	}
	if len(objects) != 3 || string(objects[0]["kind"]) != `"task"` ||
		string(objects[1]["kind"]) != `"project"` || string(objects[2]["kind"]) != `"area"` {
		t.Fatalf("JSON rows = %v, want ordered task, project, area hits", objects)
	}
	for index := range objects {
		for _, field := range []string{"task", "project", "area", "project_title", "governing_area_title"} {
			if _, ok := objects[index][field]; ok {
				t.Errorf("row %d fields = %v, want flattened row without %q", index, objects[index], field)
			}
		}
	}
	if len(objects[0]) != 15 || string(objects[0]["tags"]) != `["<repair> & upkeep"]` ||
		string(objects[0]["project_id"]) != "7" {
		t.Errorf("task fields = %v, want complete canonical task plus kind", objects[0])
	}
	if len(objects[1]) != 12 || string(objects[1]["tags"]) != `[]` ||
		string(objects[1]["area_id"]) != "4" {
		t.Errorf("project fields = %v, want complete canonical project plus kind", objects[1])
	}
	if len(objects[2]) != 9 || string(objects[2]["tags"]) != `["house"]` ||
		string(objects[2]["archived_at"]) != `"2026-08-04T12:00:00Z"` {
		t.Errorf("area fields = %v, want complete canonical area plus kind", objects[2])
	}
}

func TestSearchHumanTableShowsStatusContextAlignmentAndEscapedControls(t *testing.T) {
	t.Parallel()

	projectTitle := "Bath\x1broom"
	areaTitle := "Home\rbase"
	archivedAt := "2026-08-04T12:00:00Z"
	hits := []search.Hit{
		{Kind: search.KindTask, Task: &task.Task{ID: 2, Title: "Fix\nsink", Status: "done"}, ProjectTitle: &projectTitle, GoverningAreaTitle: &areaTitle},
		{Kind: search.KindProject, Project: &project.Project{ID: 12, Title: "Release", Status: "cancelled"}, GoverningAreaTitle: &areaTitle},
		{Kind: search.KindArea, Area: &area.Area{ID: 3, Title: "Active"}},
		{Kind: search.KindArea, Area: &area.Area{ID: 20, Title: "Cabin", ArchivedAt: &archivedAt}},
	}
	result := runSearchCommand(t, &fakeSearchApplication{result: hits}, "search", "fix*")

	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want human success", result)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	wantFields := []string{
		"kind id title status context",
		`task 2 Fix\nsink done Bath\x1broom · Home\rbase`,
		`project 12 Release cancelled Home\rbase`,
		"area 3 Active",
		"area 20 Cabin archived",
	}
	if len(lines) != len(wantFields) {
		t.Fatalf("stdout = %q, want header and four result rows", result.stdout)
	}
	for index, want := range wantFields {
		if got := strings.Join(strings.Fields(lines[index]), " "); got != want {
			t.Errorf("row %d = %q, want %q", index, got, want)
		}
	}
	if !strings.Contains(result.stdout, "\n  task      2  ") ||
		!strings.Contains(result.stdout, "\n  project  12  ") {
		t.Errorf("stdout = %q, want right-aligned IDs in the second column", result.stdout)
	}
	if strings.ContainsAny(result.stdout, "\x1b\r") {
		t.Errorf("stdout = %q, want stored controls visibly escaped", result.stdout)
	}
}

func TestSearchHumanTableAppliesFaintAndStatusStyles(t *testing.T) {
	t.Parallel()

	projectTitle := "Bathroom"
	areaTitle := "Home"
	archivedAt := "2026-08-04T12:00:00Z"
	hits := []search.Hit{
		{Kind: search.KindTask, Task: &task.Task{ID: 2, Title: "Fix sink", Status: "done"}, ProjectTitle: &projectTitle, GoverningAreaTitle: &areaTitle},
		{Kind: search.KindProject, Project: &project.Project{ID: 12, Title: "Release", Status: "cancelled"}},
		{Kind: search.KindArea, Area: &area.Area{ID: 20, Title: "Cabin", ArchivedAt: &archivedAt}},
	}
	styled := renderHuman(t, colorprofile.TrueColor, func(output humanOutput) error {
		return output.writeSearchHits(hits)
	})
	plain := renderHuman(t, colorprofile.NoTTY, func(output humanOutput) error {
		return output.writeSearchHits(hits)
	})

	if ansi.Strip(styled) != plain {
		t.Errorf("stripped styled table = %q, want plain table %q", ansi.Strip(styled), plain)
	}
	for _, line := range strings.Split(styled, "\n") {
		switch {
		case strings.Contains(line, "Fix sink"):
			if strings.Count(line, "\x1b[2m") < 3 || !strings.Contains(line, frappeGreenSGR) {
				t.Errorf("task row = %q, want faint kind/ID/context and green done status", line)
			}
		case strings.Contains(line, "Release"):
			if strings.Count(line, "\x1b[2m") < 2 || !strings.Contains(line, frappeRedSGR) {
				t.Errorf("project row = %q, want faint kind/ID and red cancelled status", line)
			}
		case strings.Contains(line, "Cabin"):
			if strings.Count(line, "\x1b[2m") < 2 || !strings.Contains(line, frappeRedSGR) {
				t.Errorf("area row = %q, want faint kind/ID and red archived status", line)
			}
		}
	}
}

func TestSearchEmptyOutputUsesModeContractAndClosesFactory(t *testing.T) {
	t.Parallel()

	humanApplication := &fakeSearchApplication{result: []search.Hit{}}
	human := runSearchCommand(t, humanApplication, "search", "nothing")
	if human.exitCode != 0 || human.stdout != "" || human.stderr != "" ||
		human.opens != 1 || human.closes != 1 || humanApplication.calls != 1 {
		t.Errorf("empty human result = %#v, want no output and one lifecycle", human)
	}

	jsonApplication := &fakeSearchApplication{result: []search.Hit{}}
	jsonResult := runSearchCommand(t, jsonApplication, "search", "nothing", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stdout != "[]\n" || jsonResult.stderr != "" ||
		jsonResult.opens != 1 || jsonResult.closes != 1 || jsonApplication.calls != 1 {
		t.Errorf("empty JSON result = %#v, want [] and one lifecycle", jsonResult)
	}
}

func TestSearchApplicationErrorsUseStderrStableExitAndCloseFactory(t *testing.T) {
	t.Parallel()

	codedApplication := &fakeSearchApplication{err: apperr.New(
		apperr.InvalidArgument,
		"invalid search expression: syntax error",
		nil,
	)}
	coded := runSearchCommand(t, codedApplication, "search", "term AND", "--json")
	if coded.exitCode != 1 || coded.stdout != "" || coded.opens != 1 || coded.closes != 1 {
		t.Fatalf("coded result = %#v, want stderr-only application failure and one lifecycle", coded)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(coded.stderr), &envelope); err != nil {
		t.Fatalf("decode JSON error: %v", err)
	}
	if envelope.Error.Code != apperr.InvalidArgument || envelope.Error.Message != "invalid search expression: syntax error" {
		t.Errorf("error = %#v, want stable invalid_argument", envelope.Error)
	}

	uncoded := runSearchCommand(t, &fakeSearchApplication{err: errors.New("search failed")}, "search", "term")
	if uncoded.exitCode != 1 || uncoded.stdout != "" || uncoded.stderr != "Error: search failed\n" ||
		uncoded.opens != 1 || uncoded.closes != 1 {
		t.Errorf("uncoded result = %#v, want normalized internal stderr error", uncoded)
	}
}

func TestSearchRequiresExactlyOneExpressionBeforeOpeningFactory(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"search"},
		{"search", "one", "two"},
	} {
		application := &fakeSearchApplication{}
		result := runSearchCommand(t, application, args...)
		if result.exitCode != 2 || result.opens != 0 || result.closes != 0 ||
			result.stdout != "" || result.stderr == "" || application.calls != 0 {
			t.Errorf("%v result = %#v, want stderr-only usage failure before factory open", args, result)
		}
	}
}

var _ search.Application = (*fakeSearchApplication)(nil)
