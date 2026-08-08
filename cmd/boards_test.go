package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
)

type fakeBoardApplication struct {
	addResult          board.Addition
	addError           error
	listResult         []board.ListedBoard
	listError          error
	showResult         board.Show
	showError          error
	editResult         board.Board
	editError          error
	reorderResult      board.Board
	reorderError       error
	deleteResult       board.Deletion
	deleteError        error
	addStageResult     board.StageResult
	addStageError      error
	renameStageResult  board.StageRenameResult
	renameStageError   error
	reorderStageResult board.StageResult
	reorderStageError  error
	deleteStageResult  board.StageResult
	deleteStageError   error
	addFields          board.AddFields
	showName           string
	editName           string
	editFields         board.EditFields
	reorderName        string
	reorderPlacement   board.Placement
	deleteName         string
	addStageBoard      string
	addStageName       string
	addStagePlacement  board.Placement
	renameStageBoard   string
	renameStageOld     string
	renameStageNew     string
	reorderStageBoard  string
	reorderStageName   string
	reorderStagePlace  board.Placement
	deleteStageBoard   string
	deleteStageName    string
}

func (f *fakeBoardApplication) Add(_ context.Context, fields board.AddFields) (board.Addition, error) {
	f.addFields = fields
	return f.addResult, f.addError
}

func (f *fakeBoardApplication) List(context.Context) ([]board.ListedBoard, error) {
	return f.listResult, f.listError
}

func (f *fakeBoardApplication) Show(_ context.Context, name string) (board.Show, error) {
	f.showName = name
	return f.showResult, f.showError
}

func (f *fakeBoardApplication) Edit(
	_ context.Context,
	name string,
	fields board.EditFields,
) (board.Board, error) {
	f.editName = name
	f.editFields = fields
	return f.editResult, f.editError
}

func (f *fakeBoardApplication) Reorder(
	_ context.Context,
	name string,
	placement board.Placement,
) (board.Board, error) {
	f.reorderName = name
	f.reorderPlacement = placement
	return f.reorderResult, f.reorderError
}

func (f *fakeBoardApplication) Delete(_ context.Context, name string) (board.Deletion, error) {
	f.deleteName = name
	return f.deleteResult, f.deleteError
}

func (f *fakeBoardApplication) AddStage(
	_ context.Context,
	boardName string,
	stageName string,
	placement board.Placement,
) (board.StageResult, error) {
	f.addStageBoard = boardName
	f.addStageName = stageName
	f.addStagePlacement = placement
	return f.addStageResult, f.addStageError
}

func (f *fakeBoardApplication) RenameStage(
	_ context.Context,
	boardName string,
	oldName string,
	newName string,
) (board.StageRenameResult, error) {
	f.renameStageBoard = boardName
	f.renameStageOld = oldName
	f.renameStageNew = newName
	return f.renameStageResult, f.renameStageError
}

func (f *fakeBoardApplication) ReorderStage(
	_ context.Context,
	boardName string,
	stageName string,
	placement board.Placement,
) (board.StageResult, error) {
	f.reorderStageBoard = boardName
	f.reorderStageName = stageName
	f.reorderStagePlace = placement
	return f.reorderStageResult, f.reorderStageError
}

func (f *fakeBoardApplication) DeleteStage(
	_ context.Context,
	boardName string,
	stageName string,
) (board.StageResult, error) {
	f.deleteStageBoard = boardName
	f.deleteStageName = stageName
	return f.deleteStageResult, f.deleteStageError
}

func runBoardCommand(t *testing.T, application board.Application, args ...string) commandResult {
	t.Helper()
	return runBoardCommandWithInput(t, application, strings.NewReader(""), args...)
}

func runBoardCommandWithInput(
	t *testing.T,
	application board.Application,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{boards: application}, input, args...)
}

func decodeBoardJSON[T any](t *testing.T, output string) T {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("output = %q, want one newline-terminated JSON value", output)
	}
	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode board JSON: %v", err)
	}
	return decoded
}

func TestBoardAddAdaptsFieldsStdinAndOutputModes(t *testing.T) {
	t.Parallel()

	note := "first line\nsecond line\n"
	addition := board.Addition{
		Board: board.Board{ID: 1, Title: "software", Note: note, Position: 2},
		Stages: []board.Stage{
			{ID: 2, BoardID: 1, Title: "research", Position: 1},
			{ID: 3, BoardID: 1, Title: "planning", Position: 2},
		},
	}
	application := &fakeBoardApplication{addResult: addition}
	result := runBoardCommandWithInput(
		t,
		application,
		strings.NewReader(note),
		"boards", "add", "software", "--stage", "research", "--stage", "planning", "--note", "-", "--json",
	)
	if result.exitCode != 0 || result.stderr != "" || result.opens != 1 || result.closes != 1 {
		t.Fatalf("result = %#v, want JSON success and one open/close", result)
	}
	wantFields := board.AddFields{Title: "software", Note: note, Stages: []string{"research", "planning"}}
	if !reflect.DeepEqual(application.addFields, wantFields) {
		t.Errorf("Add() fields = %#v, want %#v", application.addFields, wantFields)
	}
	if got := decodeBoardJSON[board.Board](t, result.stdout); !reflect.DeepEqual(got, addition.Board) {
		t.Errorf("JSON = %#v, want bare board %#v", got, addition.Board)
	}

	humanAddition := board.Addition{
		Board:  board.Board{Title: "soft\x1bware"},
		Stages: []board.Stage{{Title: "research"}, {Title: "plan\nning"}},
	}
	human := runBoardCommand(t, &fakeBoardApplication{addResult: humanAddition}, "boards", "add", "ignored")
	if human.exitCode != 0 || human.stderr != "" || human.stdout != "+ Board: soft\\x1bware (research → plan\\nning)\n" {
		t.Errorf("human result = %#v, want exact escaped board creation", human)
	}
	if human.opens != 1 || human.closes != 1 {
		t.Errorf("human lifecycle = %#v, want one open/close", human)
	}
}

func TestBoardAddWithoutStageReachesServiceAndStdinFailureDoesNotOpen(t *testing.T) {
	t.Parallel()

	application := &fakeBoardApplication{addResult: board.Addition{Board: board.Board{Title: "software"}}}
	result := runBoardCommand(t, application, "boards", "add", "software", "--json")
	if result.exitCode != 0 || result.opens != 1 || result.closes != 1 {
		t.Fatalf("result = %#v, want semantic stage requirement delegated to service", result)
	}
	if application.addFields.Stages != nil {
		t.Errorf("stages = %#v, want omitted stage slice passed through", application.addFields.Stages)
	}

	failed := runBoardCommandWithInput(
		t,
		&fakeBoardApplication{},
		failingReader{},
		"boards", "add", "software", "--note", "-", "--json",
	)
	if failed.exitCode != 1 || failed.opens != 0 || failed.closes != 0 || failed.stdout != "" {
		t.Errorf("stdin failure = %#v, want internal error before open", failed)
	}
}

func TestBoardListWritesStableArraysAndAlignedHumanRows(t *testing.T) {
	t.Parallel()

	listed := []board.ListedBoard{
		{Board: board.Board{ID: 1, Title: "software"}, Stages: []board.Stage{{Title: "research"}, {Title: "planning"}}},
		{Board: board.Board{ID: 2, Title: "life"}, Stages: []board.Stage{}},
	}
	jsonResult := runBoardCommand(t, &fakeBoardApplication{listResult: listed}, "boards", "list", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stderr != "" || jsonResult.opens != 1 || jsonResult.closes != 1 {
		t.Fatalf("JSON result = %#v, want success", jsonResult)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonResult.stdout), &rows); err != nil {
		t.Fatalf("decode rows: %v", err)
	}
	if len(rows) != 2 || string(rows[0]["stages"]) == "null" || string(rows[1]["stages"]) != "[]" {
		t.Errorf("rows = %s, want embedded non-null stage arrays", jsonResult.stdout)
	}

	empty := runBoardCommand(
		t,
		&fakeBoardApplication{listResult: []board.ListedBoard{}},
		"boards", "list", "--json",
	)
	if empty.exitCode != 0 || empty.stdout != "[]\n" || empty.stderr != "" {
		t.Errorf("empty result = %#v, want empty JSON array", empty)
	}

	human := runBoardCommand(t, &fakeBoardApplication{listResult: listed}, "boards", "list")
	lines := strings.Split(strings.TrimSuffix(human.stdout, "\n"), "\n")
	if human.exitCode != 0 || human.stderr != "" || len(lines) != 3 ||
		strings.Join(strings.Fields(lines[0]), " ") != "board stages" ||
		strings.Join(strings.Fields(lines[1]), " ") != "software research → planning" ||
		strings.Join(strings.Fields(lines[2]), " ") != "life" {
		t.Errorf("human result = %#v, want aligned board/stages collection", human)
	}
	noRows := runBoardCommand(
		t,
		&fakeBoardApplication{listResult: []board.ListedBoard{}},
		"boards", "list",
	)
	if noRows.exitCode != 0 || noRows.stdout != "" || noRows.stderr != "" {
		t.Errorf("empty human result = %#v, want no output", noRows)
	}
}

func TestBoardShowWritesEnvelopeAndEmptyStageHumanView(t *testing.T) {
	t.Parallel()

	shown := board.Show{
		Board: board.Board{ID: 1, Title: "software"},
		Stages: []board.ShownStage{
			{Stage: board.Stage{ID: 2, BoardID: 1, Title: "research"}, Projects: []board.ShownProject{}},
			{Stage: board.Stage{ID: 3, BoardID: 1, Title: "planning"}, Projects: []board.ShownProject{}},
		},
	}
	application := &fakeBoardApplication{showResult: shown}
	jsonResult := runBoardCommand(t, application, "board", "show", "SOFTWARE", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stderr != "" || application.showName != "SOFTWARE" ||
		jsonResult.opens != 1 || jsonResult.closes != 1 {
		t.Fatalf("JSON result = %#v/name %q, want exact adaptation", jsonResult, application.showName)
	}
	var envelope struct {
		Board  board.Board `json:"board"`
		Stages []struct {
			Projects json.RawMessage `json:"projects"`
		} `json:"stages"`
	}
	if err := json.Unmarshal([]byte(jsonResult.stdout), &envelope); err != nil {
		t.Fatalf("decode show: %v", err)
	}
	if envelope.Board.Title != "software" || len(envelope.Stages) != 2 ||
		string(envelope.Stages[0].Projects) != "[]" || string(envelope.Stages[1].Projects) != "[]" {
		t.Errorf("show = %s, want board envelope with non-null project arrays", jsonResult.stdout)
	}

	human := runBoardCommand(t, &fakeBoardApplication{showResult: shown}, "board", "show", "software")
	want := "software  research → planning\n  research  (empty)\n  planning  (empty)\n"
	if human.exitCode != 0 || human.stdout != want || human.stderr != "" {
		t.Errorf("human result = %#v, want %q", human, want)
	}

	zero := runBoardCommand(t, &fakeBoardApplication{showResult: board.Show{
		Board:  board.Board{Title: "soft\x1bware"},
		Stages: []board.ShownStage{},
	}}, "board", "show", "software")
	if zero.exitCode != 0 || zero.stdout != "soft\\x1bware  (no stages)\n" || zero.stderr != "" {
		t.Errorf("zero-stage result = %#v, want escaped no-stages headline", zero)
	}
}

func TestBoardShowWritesPopulatedProgressEnvelopeAndHumanColumns(t *testing.T) {
	t.Parallel()

	shown := board.Show{
		Board: board.Board{ID: 1, Title: "software"},
		Stages: []board.ShownStage{
			{Stage: board.Stage{ID: 2, BoardID: 1, Title: "research"}, Projects: []board.ShownProject{}},
			{Stage: board.Stage{ID: 3, BoardID: 1, Title: "planning"}, Projects: []board.ShownProject{
				{Project: project.Project{ID: 14, Title: "homelab backups", Tags: domain.TagNames{}}, Progress: board.ProjectProgress{Done: 2, Total: 6}},
			}},
			{Stage: board.Stage{ID: 4, BoardID: 1, Title: "doing"}, Projects: []board.ShownProject{
				{Project: project.Project{ID: 12, Title: "gsd boards milestone", Tags: domain.TagNames{}}, Progress: board.ProjectProgress{Done: 5, Total: 8}},
				{Project: project.Project{ID: 9, Title: "blog rewrite", Tags: domain.TagNames{}}, Progress: board.ProjectProgress{Done: 1, Total: 3}},
			}},
			{Stage: board.Stage{ID: 5, BoardID: 1, Title: "review"}, Projects: []board.ShownProject{}},
		},
	}
	jsonResult := runBoardCommand(t, &fakeBoardApplication{showResult: shown},
		"board", "show", "software", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stderr != "" {
		t.Fatalf("JSON result = %#v, want success", jsonResult)
	}
	got := decodeBoardJSON[board.Show](t, jsonResult.stdout)
	if !reflect.DeepEqual(got, shown) {
		t.Errorf("JSON show = %#v, want populated envelope %#v", got, shown)
	}

	human := runBoardCommand(t, &fakeBoardApplication{showResult: shown}, "board", "show", "software")
	wantRows := []string{
		"software research → planning → doing → review",
		"research (empty)",
		"planning ◆ 14 homelab backups 2/6",
		"doing ◆ 12 gsd boards milestone 5/8",
		"◆ 9 blog rewrite 1/3",
		"review (empty)",
	}
	lines := strings.Split(strings.TrimSuffix(human.stdout, "\n"), "\n")
	if human.exitCode != 0 || human.stderr != "" || len(lines) != len(wantRows) {
		t.Fatalf("human result = %#v, want %d board rows", human, len(wantRows))
	}
	for index, want := range wantRows {
		if got := humanFields(lines[index]); got != want {
			t.Errorf("human row %d = %q, want %q", index, got, want)
		}
	}
}

func TestBoardEditAndDeleteAdaptExactEnvelopesAndHumanOutput(t *testing.T) {
	t.Parallel()

	note := "pipeline\n"
	edited := board.Board{ID: 1, Title: "Software", Note: note}
	editApplication := &fakeBoardApplication{editResult: edited}
	edit := runBoardCommandWithInput(
		t,
		editApplication,
		strings.NewReader(note),
		"board", "edit", "software", "--title", "Software", "--note", "-", "--json",
	)
	if edit.exitCode != 0 || edit.stderr != "" || editApplication.editName != "software" ||
		editApplication.editFields.Title == nil || *editApplication.editFields.Title != "Software" ||
		editApplication.editFields.Note == nil || *editApplication.editFields.Note != note {
		t.Fatalf("edit/result = %#v/%#v, want exact fields", edit, editApplication.editFields)
	}
	if got := decodeBoardJSON[board.Board](t, edit.stdout); !reflect.DeepEqual(got, edited) {
		t.Errorf("edit JSON = %#v, want bare board %#v", got, edited)
	}
	editHuman := runBoardCommand(t, &fakeBoardApplication{editResult: edited},
		"board", "edit", "software", "--title", "Software")
	if editHuman.exitCode != 0 || editHuman.stdout != "~ Edited: board Software\n" || editHuman.stderr != "" {
		t.Errorf("human edit = %#v, want exact mutation", editHuman)
	}
	reorderHuman := runBoardCommand(t, &fakeBoardApplication{reorderResult: edited},
		"board", "reorder", "software", "--first")
	if reorderHuman.exitCode != 0 || reorderHuman.stdout != "~ Reordered: board Software\n" || reorderHuman.stderr != "" {
		t.Errorf("human reorder = %#v, want exact mutation", reorderHuman)
	}

	deletion := board.Deletion{Board: board.Board{ID: 1, Title: "soft\rware"}, Stages: []board.Stage{}}
	deleteApplication := &fakeBoardApplication{deleteResult: deletion}
	deletedJSON := runBoardCommand(t, deleteApplication, "board", "delete", "software", "--json")
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(deletedJSON.stdout), &fields); err != nil {
		t.Fatalf("decode deletion: %v", err)
	}
	if deletedJSON.exitCode != 0 || deletedJSON.stderr != "" || deleteApplication.deleteName != "software" ||
		string(fields["stages"]) != "[]" {
		t.Errorf("delete result = %#v/%s, want full deletion with stage array", deletedJSON, deletedJSON.stdout)
	}
	deletedHuman := runBoardCommand(t, &fakeBoardApplication{deleteResult: deletion}, "board", "delete", "software")
	if deletedHuman.exitCode != 0 || deletedHuman.stdout != "− Deleted: board soft\\rware\n" || deletedHuman.stderr != "" {
		t.Errorf("human deletion = %#v, want exact escaped mutation", deletedHuman)
	}
}

func TestBoardAndStagePlacementsAdaptNamesExactly(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		flag string
		want board.Placement
	}{
		{name: "after", flag: "--after=Other Board", want: board.Placement{Anchor: domain.PlacementAfter, Reference: "Other Board"}},
		{name: "before", flag: "--before=  Earlier  ", want: board.Placement{Anchor: domain.PlacementBefore, Reference: "  Earlier  "}},
		{name: "first", flag: "--first", want: board.Placement{Anchor: domain.PlacementFirst}},
		{name: "last", flag: "--last", want: board.Placement{Anchor: domain.PlacementLast}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			application := &fakeBoardApplication{reorderResult: board.Board{Title: "Software"}}
			result := runBoardCommand(t, application, "board", "reorder", "software", test.flag, "--json")
			if result.exitCode != 0 || result.stderr != "" || result.opens != 1 || result.closes != 1 {
				t.Fatalf("result = %#v, want success", result)
			}
			if application.reorderName != "software" || !reflect.DeepEqual(application.reorderPlacement, test.want) {
				t.Errorf("Reorder() = %q/%#v, want software/%#v", application.reorderName, application.reorderPlacement, test.want)
			}
			if got := decodeBoardJSON[board.Board](t, result.stdout); got.Title != "Software" {
				t.Errorf("JSON = %#v, want bare reordered board", got)
			}
		})
	}

	addApplication := &fakeBoardApplication{addStageResult: board.StageResult{
		Board: board.Board{Title: "Software"}, Stage: board.Stage{Title: "Intake"},
	}}
	added := runBoardCommand(t, addApplication, "stages", "add", "software", "intake")
	if added.exitCode != 0 || added.stdout != "+ Added stage Software/Intake\n" || added.stderr != "" ||
		addApplication.addStageBoard != "software" || addApplication.addStageName != "intake" ||
		addApplication.addStagePlacement != (board.Placement{}) {
		t.Errorf("stage add = %#v/call %#v, want optional empty placement", added, addApplication)
	}
	addedJSON := runBoardCommand(t, &fakeBoardApplication{addStageResult: addApplication.addStageResult},
		"stages", "add", "software", "intake", "--last", "--json")
	if got := decodeBoardJSON[board.Stage](t, addedJSON.stdout); !reflect.DeepEqual(got, addApplication.addStageResult.Stage) {
		t.Errorf("add JSON = %#v, want bare stage %#v", got, addApplication.addStageResult.Stage)
	}

	reorderApplication := &fakeBoardApplication{reorderStageResult: board.StageResult{
		Board: board.Board{Title: "Software"}, Stage: board.Stage{Title: "Doing"},
	}}
	reordered := runBoardCommand(t, reorderApplication, "stage", "reorder", "software", "doing", "--before", "review")
	wantPlacement := board.Placement{Anchor: domain.PlacementBefore, Reference: "review"}
	if reordered.exitCode != 0 || reordered.stdout != "~ Reordered: stage Software/Doing\n" || reordered.stderr != "" ||
		reorderApplication.reorderStageBoard != "software" || reorderApplication.reorderStageName != "doing" ||
		!reflect.DeepEqual(reorderApplication.reorderStagePlace, wantPlacement) {
		t.Errorf("stage reorder = %#v/call %#v, want exact name placement", reordered, reorderApplication)
	}
	reorderedJSON := runBoardCommand(t, &fakeBoardApplication{reorderStageResult: reorderApplication.reorderStageResult},
		"stage", "reorder", "software", "doing", "--first", "--json")
	if got := decodeBoardJSON[board.Stage](t, reorderedJSON.stdout); !reflect.DeepEqual(got, reorderApplication.reorderStageResult.Stage) {
		t.Errorf("reorder JSON = %#v, want bare stage %#v", got, reorderApplication.reorderStageResult.Stage)
	}
}

func TestStageMutationsWriteBareStageJSONAndStoredHumanNames(t *testing.T) {
	t.Parallel()

	renaming := board.StageRenameResult{
		Board:         board.Board{Title: "Software"},
		Stage:         board.Stage{ID: 3, BoardID: 1, Title: "Triage"},
		PreviousTitle: "In\x1btake",
	}
	renameApplication := &fakeBoardApplication{renameStageResult: renaming}
	renamed := runBoardCommand(t, renameApplication, "stage", "rename", "software", "intake", "triage")
	if renamed.exitCode != 0 || renamed.stderr != "" ||
		renamed.stdout != "~ Renamed stage Software/In\\x1btake to Software/Triage\n" ||
		renameApplication.renameStageBoard != "software" || renameApplication.renameStageOld != "intake" ||
		renameApplication.renameStageNew != "triage" || renamed.opens != 1 || renamed.closes != 1 {
		t.Errorf("rename = %#v/call %#v, want stored escaped names and one lifecycle", renamed, renameApplication)
	}

	renameJSON := runBoardCommand(t, &fakeBoardApplication{renameStageResult: renaming},
		"stage", "rename", "software", "intake", "triage", "--json")
	if got := decodeBoardJSON[board.Stage](t, renameJSON.stdout); !reflect.DeepEqual(got, renaming.Stage) {
		t.Errorf("rename JSON = %#v, want bare stage %#v", got, renaming.Stage)
	}

	deletedResult := board.StageResult{
		Board: board.Board{Title: "Software"}, Stage: board.Stage{ID: 3, BoardID: 1, Title: "Triage\r"},
	}
	deleteApplication := &fakeBoardApplication{deleteStageResult: deletedResult}
	deleted := runBoardCommand(t, deleteApplication, "stage", "delete", "software", "triage")
	if deleted.exitCode != 0 || deleted.stderr != "" ||
		deleted.stdout != "− Deleted: stage Software/Triage\\r\n" ||
		deleteApplication.deleteStageBoard != "software" || deleteApplication.deleteStageName != "triage" {
		t.Errorf("delete = %#v/call %#v, want stored escaped stage", deleted, deleteApplication)
	}
	deletedJSON := runBoardCommand(t, &fakeBoardApplication{deleteStageResult: deletedResult},
		"stage", "delete", "software", "triage", "--json")
	if got := decodeBoardJSON[board.Stage](t, deletedJSON.stdout); !reflect.DeepEqual(got, deletedResult.Stage) {
		t.Errorf("delete JSON = %#v, want bare stage %#v", got, deletedResult.Stage)
	}
}

func TestBoardPlacementGrammarFailuresDoNotOpenFactory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "board reorder missing", args: []string{"board", "reorder", "software", "--json"}},
		{name: "board reorder duplicate", args: []string{"board", "reorder", "software", "--first", "--last", "--json"}},
		{name: "board reorder false", args: []string{"board", "reorder", "software", "--first=false", "--json"}},
		{name: "stage reorder missing", args: []string{"stage", "reorder", "software", "doing", "--json"}},
		{name: "stage reorder duplicate", args: []string{"stage", "reorder", "software", "doing", "--after", "one", "--before", "two", "--json"}},
		{name: "stage reorder false", args: []string{"stage", "reorder", "software", "doing", "--last=false", "--json"}},
		{name: "stage add duplicate", args: []string{"stages", "add", "software", "doing", "--first", "--after", "one", "--json"}},
		{name: "stage add first false", args: []string{"stages", "add", "software", "doing", "--first=false", "--json"}},
		{name: "stage add last false", args: []string{"stages", "add", "software", "doing", "--last=false", "--json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := runBoardCommand(t, &fakeBoardApplication{}, test.args...)
			if result.exitCode != 2 || result.opens != 0 || result.closes != 0 || result.stdout != "" || result.stderr == "" {
				t.Errorf("result = %#v, want stderr-only usage error without lifecycle", result)
			}
			if strings.HasPrefix(result.stderr, "{") {
				t.Errorf("stderr = %q, want human Cobra/usage diagnostic", result.stderr)
			}
		})
	}
}

func TestBoardParentsLeafHelpAndArityNeverOpenFactory(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"boards", "--help"},
		{"board", "--help"},
		{"stages", "--help"},
		{"stage", "--help"},
		{"boards", "add", "--help"},
		{"boards", "list", "--help"},
		{"board", "show", "--help"},
		{"board", "edit", "--help"},
		{"board", "reorder", "--help"},
		{"board", "delete", "--help"},
		{"stages", "add", "--help"},
		{"stage", "rename", "--help"},
		{"stage", "reorder", "--help"},
		{"stage", "delete", "--help"},
	} {
		result := runBoardCommand(t, &fakeBoardApplication{}, args...)
		if result.exitCode != 0 || result.opens != 0 || result.closes != 0 || result.stderr != "" || result.stdout == "" {
			t.Errorf("help %v = %#v, want stdout help without lifecycle", args, result)
		}
	}

	for _, args := range [][]string{
		{"boards", "add"},
		{"boards", "list", "extra"},
		{"board", "show"},
		{"board", "edit"},
		{"board", "reorder", "--first"},
		{"board", "delete"},
		{"stages", "add", "software"},
		{"stage", "rename", "software", "old"},
		{"stage", "reorder", "software", "--first"},
		{"stage", "delete", "software"},
	} {
		result := runBoardCommand(t, &fakeBoardApplication{}, args...)
		if result.exitCode != 2 || result.opens != 0 || result.closes != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("arity %v = %#v, want stderr-only usage failure without lifecycle", args, result)
		}
	}

	for _, noun := range []string{"boards", "board", "stages", "stage"} {
		result := runBoardCommand(t, &fakeBoardApplication{}, noun)
		if result.exitCode != 2 || result.opens != 0 || result.closes != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("parent %s = %#v, want usage failure without lifecycle", noun, result)
		}
	}
}

func TestBoardEditWithoutFieldsAndApplicationErrorsMapToOwnedExits(t *testing.T) {
	t.Parallel()

	noFields := runBoardCommand(t, &fakeBoardApplication{}, "board", "edit", "software", "--json")
	if noFields.exitCode != 1 || noFields.opens != 0 || noFields.closes != 0 || noFields.stdout != "" {
		t.Fatalf("no-fields result = %#v, want invalid_argument before open", noFields)
	}
	noFieldsError := decodeBoardJSON[errorEnvelope](t, noFields.stderr).Error
	if noFieldsError.Code != apperr.InvalidArgument || !strings.Contains(noFieldsError.Message, "--title") {
		t.Errorf("error = %#v, want edit flag guidance", noFieldsError)
	}

	missing := runBoardCommand(t, &fakeBoardApplication{showError: apperr.New(
		apperr.NotFound,
		"board Missing not found",
		nil,
	)}, "board", "show", "Missing", "--json")
	if missing.exitCode != 1 || missing.opens != 1 || missing.closes != 1 || missing.stdout != "" {
		t.Fatalf("missing result = %#v, want application failure and one open/close", missing)
	}
	missingError := decodeBoardJSON[errorEnvelope](t, missing.stderr).Error
	if missingError.Code != apperr.NotFound || missingError.Message != "board Missing not found" {
		t.Errorf("error = %#v, want stable not_found", missingError)
	}

	internal := runBoardCommand(t, &fakeBoardApplication{listError: errors.New("read boards failed")},
		"boards", "list", "--json")
	if internal.exitCode != 1 || internal.opens != 1 || internal.closes != 1 || internal.stdout != "" {
		t.Fatalf("internal result = %#v, want internal application failure", internal)
	}
	internalError := decodeBoardJSON[errorEnvelope](t, internal.stderr).Error
	if internalError.Code != apperr.Internal || internalError.Message != "read boards failed" {
		t.Errorf("error = %#v, want normalized internal", internalError)
	}
}

func TestBoardNamesRemainServiceValidatedAfterFactoryOpen(t *testing.T) {
	t.Parallel()

	application := &fakeBoardApplication{showResult: board.Show{Board: board.Board{Title: "stored"}}}
	result := runBoardCommand(t, application, "board", "show", " ", "--json")
	if result.exitCode != 0 || result.opens != 1 || result.closes != 1 || application.showName != " " {
		t.Errorf("result/call = %#v/%q, want exact unvalidated name passed after open", result, application.showName)
	}
}

var _ board.Application = (*fakeBoardApplication)(nil)
