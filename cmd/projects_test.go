package cmd

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeProjectApplication struct {
	addResult        project.Project
	addError         error
	listResult       []project.Project
	listError        error
	showResult       project.Detail
	showError        error
	editResult       project.Edition
	editError        error
	moveResult       project.Movement
	moveError        error
	resolveResult    project.Resolution
	resolveError     error
	reopenResult     project.Project
	reopenError      error
	reorderResult    project.Project
	reorderError     error
	tagResult        project.Tagging
	tagError         error
	untagResult      project.Tagging
	untagError       error
	deleteResult     project.Deletion
	deleteError      error
	addFields        project.AddRequest
	listOptions      project.ListOptions
	showID           int64
	editID           int64
	editFields       project.EditRequest
	moveID           int64
	moveStage        string
	movePlacement    *domain.Placement
	resolveID        int64
	resolveExit      project.Exit
	reopenID         int64
	reorderID        int64
	reorderPlacement domain.Placement
	tagID            int64
	tagNames         []string
	untagID          int64
	untagNames       []string
	deleteID         int64
	deleteRecursive  bool
}

func (f *fakeProjectApplication) Add(
	_ context.Context,
	fields project.AddRequest,
) (project.Project, error) {
	f.addFields = fields
	return f.addResult, f.addError
}

func (f *fakeProjectApplication) List(
	_ context.Context,
	options project.ListOptions,
) ([]project.Project, error) {
	f.listOptions = options
	return f.listResult, f.listError
}

func (f *fakeProjectApplication) Show(
	_ context.Context,
	id int64,
) (project.Detail, error) {
	f.showID = id
	return f.showResult, f.showError
}

func (f *fakeProjectApplication) Edit(
	_ context.Context,
	id int64,
	fields project.EditRequest,
) (project.Edition, error) {
	f.editID = id
	f.editFields = fields
	return f.editResult, f.editError
}

func (f *fakeProjectApplication) Move(
	_ context.Context,
	id int64,
	stage string,
	placement *domain.Placement,
) (project.Movement, error) {
	f.moveID = id
	f.moveStage = stage
	f.movePlacement = placement
	return f.moveResult, f.moveError
}

func (f *fakeProjectApplication) Resolve(
	_ context.Context,
	id int64,
	exit project.Exit,
) (project.Resolution, error) {
	f.resolveID = id
	f.resolveExit = exit
	return f.resolveResult, f.resolveError
}

func (f *fakeProjectApplication) Reopen(
	_ context.Context,
	id int64,
) (project.Project, error) {
	f.reopenID = id
	return f.reopenResult, f.reopenError
}

func (f *fakeProjectApplication) Reorder(
	_ context.Context,
	id int64,
	placement domain.Placement,
) (project.Project, error) {
	f.reorderID = id
	f.reorderPlacement = placement
	return f.reorderResult, f.reorderError
}

func (f *fakeProjectApplication) Tag(
	_ context.Context,
	id int64,
	names []string,
) (project.Tagging, error) {
	f.tagID = id
	f.tagNames = names
	return f.tagResult, f.tagError
}

func (f *fakeProjectApplication) Untag(
	_ context.Context,
	id int64,
	names []string,
) (project.Tagging, error) {
	f.untagID = id
	f.untagNames = names
	return f.untagResult, f.untagError
}

func (f *fakeProjectApplication) Delete(
	_ context.Context,
	id int64,
	recursive bool,
) (project.Deletion, error) {
	f.deleteID = id
	f.deleteRecursive = recursive
	return f.deleteResult, f.deleteError
}

func decodeProjectJSON[T any](t *testing.T, output string) T {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("output = %q, want one newline-terminated JSON value", output)
	}

	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode project command output: %v", err)
	}
	return decoded
}

func requireProjectCommandJSON[T any](t *testing.T, result commandResult, want T) {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want JSON success", result)
	}
	if got := decodeProjectJSON[T](t, result.stdout); !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON output = %#v, want %#v", got, want)
	}
}

func decodeProjectCommandError(t *testing.T, result commandResult) errorPayload {
	t.Helper()
	if result.exitCode != 1 || result.stdout != "" {
		t.Fatalf("result = %#v, want stderr-only application error", result)
	}
	return decodeProjectJSON[errorEnvelope](t, result.stderr).Error
}

func requireProjectCommandHumanOutput(t *testing.T, result commandResult, fragments ...string) {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" {
		t.Errorf("result = %#v, want human success", result)
	}
	normalized := humanFields(result.stdout)
	for _, fragment := range fragments {
		if !strings.Contains(normalized, fragment) {
			t.Errorf("stdout = %q, want %q", result.stdout, fragment)
		}
	}
}

func TestTaskProjectFlagsAdaptContainmentIntent(t *testing.T) {
	t.Parallel()

	projectID := int64(7)
	addApplication := &fakeApplication{addResult: task.Task{
		ID:        1,
		ProjectID: &projectID,
		Title:     "contained",
	}}
	addResult := runCommand(t, addApplication, "add", "contained", "--project", "007", "--json")
	if addResult.exitCode != 0 || addResult.stderr != "" {
		t.Fatalf("add result = %#v, want success", addResult)
	}
	if addApplication.addProjectID == nil || *addApplication.addProjectID != projectID {
		t.Errorf("Add() project ID = %#v, want 7", addApplication.addProjectID)
	}
	var added task.Task
	if err := json.Unmarshal([]byte(addResult.stdout), &added); err != nil {
		t.Fatalf("decode add output: %v", err)
	}
	if added.ProjectID == nil || *added.ProjectID != projectID {
		t.Errorf("JSON project_id = %#v, want 7", added.ProjectID)
	}

	listApplication := &fakeApplication{listResult: []task.Task{}}
	listResult := runCommand(t, listApplication, "list", "--project", "8", "--json")
	if listResult.exitCode != 0 || listResult.stderr != "" || listResult.stdout != "[]\n" {
		t.Fatalf("list result = %#v, want empty success", listResult)
	}
	if listApplication.listOptions.ProjectID == nil || *listApplication.listOptions.ProjectID != 8 {
		t.Errorf("List() project ID = %#v, want 8", listApplication.listOptions.ProjectID)
	}

	setApplication := &fakeApplication{editResult: task.Task{ID: 3, Title: "moved"}}
	setResult := runCommand(t, setApplication, "edit", "3", "--project", "9", "--json")
	if setResult.exitCode != 0 || setResult.stderr != "" {
		t.Fatalf("set result = %#v, want success", setResult)
	}
	if setApplication.editFields.Project.Set == nil ||
		*setApplication.editFields.Project.Set != 9 || setApplication.editFields.Project.Clear {
		t.Errorf("Edit() project change = %#v, want set 9", setApplication.editFields.Project)
	}

	clearApplication := &fakeApplication{editResult: task.Task{ID: 3, Title: "loose"}}
	clearResult := runCommand(t, clearApplication, "edit", "3", "--no-project", "--json")
	if clearResult.exitCode != 0 || clearResult.stderr != "" {
		t.Fatalf("clear result = %#v, want success", clearResult)
	}
	if !clearApplication.editFields.Project.Clear || clearApplication.editFields.Project.Set != nil {
		t.Errorf("Edit() project change = %#v, want clear", clearApplication.editFields.Project)
	}
}

func TestTaskAreaFlagsAdaptContainmentIntent(t *testing.T) {
	t.Parallel()

	areaID := int64(8)
	addApplication := &fakeApplication{addResult: task.Task{ID: 1, AreaID: &areaID, Title: "contained"}}
	addResult := runCommand(t, addApplication, "add", "contained", "--area", "008", "--json")
	if addResult.exitCode != 0 || addApplication.addAreaID == nil || *addApplication.addAreaID != areaID {
		t.Fatalf("add result/input = %#v/%#v, want area 8 success", addResult, addApplication.addAreaID)
	}

	listApplication := &fakeApplication{listResult: []task.Task{}}
	listResult := runCommand(t, listApplication, "list", "--area", "9", "--json")
	if listResult.exitCode != 0 || listApplication.listOptions.AreaID == nil ||
		*listApplication.listOptions.AreaID != 9 {
		t.Fatalf("list result/options = %#v/%#v, want area 9", listResult, listApplication.listOptions)
	}

	setApplication := &fakeApplication{editResult: task.Task{ID: 3}}
	setResult := runCommand(t, setApplication, "edit", "3", "--area", "10", "--no-project", "--json")
	if setResult.exitCode != 0 || setApplication.editFields.Area.Set == nil ||
		*setApplication.editFields.Area.Set != 10 || !setApplication.editFields.Project.Clear {
		t.Fatalf("cross-clear edit = %#v/%#v, want area set and project clear", setResult, setApplication.editFields)
	}

	clearApplication := &fakeApplication{editResult: task.Task{ID: 3}}
	clearResult := runCommand(t, clearApplication, "edit", "3", "--no-area", "--project", "11", "--json")
	if clearResult.exitCode != 0 || !clearApplication.editFields.Area.Clear ||
		clearApplication.editFields.Project.Set == nil || *clearApplication.editFields.Project.Set != 11 {
		t.Fatalf("cross-clear edit = %#v/%#v, want area clear and project set", clearResult, clearApplication.editFields)
	}
}

func TestTaskAreaGrammarAndValidationHappenBeforeApplicationOpen(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"add", "capture", "--area", "0", "--json"},
		{"list", "--area", "nope", "--json"},
	} {
		result := runCommand(t, &fakeApplication{}, args...)
		if result.exitCode != 1 || result.opens != 0 || result.stdout != "" {
			t.Errorf("result = %#v, want invalid_argument without open", result)
		}
	}

	result := runCommand(t, &fakeApplication{}, "edit", "3", "--area", "7", "--no-area", "--json")
	if result.exitCode != 2 || result.opens != 0 || result.stdout != "" {
		t.Errorf("result = %#v, want usage error without open", result)
	}
}

func TestTaskCrossContainerInputsReachServiceValidation(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{addError: apperr.New(apperr.InvalidArgument, "task cannot belong to both a project and an area", nil)}
	result := runCommand(t, application, "add", "capture", "--project", "7", "--area", "8", "--json")
	if result.exitCode != 1 || result.opens != 1 || application.addProjectID == nil ||
		*application.addProjectID != 7 || application.addAreaID == nil || *application.addAreaID != 8 {
		t.Errorf("result/input = %#v/(%#v, %#v), want service invalid_argument", result, application.addProjectID, application.addAreaID)
	}

	listApplication := &fakeApplication{listError: apperr.New(apperr.InvalidArgument, "cannot filter by both project and area", nil)}
	listResult := runCommand(t, listApplication, "list", "--project", "7", "--area", "8", "--json")
	if listResult.exitCode != 1 || listResult.opens != 1 || listApplication.listOptions.ProjectID == nil ||
		listApplication.listOptions.AreaID == nil {
		t.Errorf("list result/options = %#v/%#v, want service invalid_argument", listResult, listApplication.listOptions)
	}
}

func TestTaskProjectFlagRejectsNonDecimalIDAsApplicationErrorWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{}, "add", "contained", "--project", "0x8", "--json")
	if result.exitCode != 1 || result.opens != 0 || result.stdout != "" {
		t.Errorf("result = %#v, want stderr-only invalid_argument without open", result)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != apperr.InvalidArgument {
		t.Errorf("error code = %q, want invalid_argument", envelope.Error.Code)
	}
}

func TestTaskProjectFlagConflictIsUsageWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	result := runCommand(
		t,
		&fakeApplication{},
		"edit",
		"3",
		"--project",
		"7",
		"--no-project",
		"--json",
	)
	if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
		t.Errorf("result = %#v, want stderr-only usage error without open", result)
	}
	if strings.HasPrefix(result.stderr, "{") {
		t.Errorf("stderr = %q, want human-readable usage diagnostic", result.stderr)
	}
}

func TestBareProjectParentsAreUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	for _, noun := range []string{"projects", "project"} {
		noun := noun
		t.Run(noun, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, &fakeApplication{}, noun, "--json")
			if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
				t.Errorf("result = %#v, want stderr-only usage error without open", result)
			}
			if strings.HasPrefix(result.stderr, "{") {
				t.Errorf("stderr = %q, want human-readable usage diagnostic", result.stderr)
			}
		})
	}
}

func TestProjectAddAndEditAdaptFieldsAndOutput(t *testing.T) {
	t.Parallel()

	note := "line one\nline two\n"
	created := project.Project{ID: 7, Title: "Kitchen reno", Note: note, Status: "open", Tags: []string{}}
	addApplication := &fakeProjectApplication{addResult: created}
	addResult := runProjectCommandWithInput(
		t,
		addApplication,
		strings.NewReader(note),
		"projects",
		"add",
		"Kitchen reno",
		"--note",
		"-",
		"--db",
		"chosen.db",
		"--json",
	)
	requireProjectCommandJSON(t, addResult, created)
	if !reflect.DeepEqual(addApplication.addFields, project.AddRequest{Title: "Kitchen reno", Note: note}) {
		t.Errorf("Add() fields = %#v, want exact title and stdin note", addApplication.addFields)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(addResult.stdout), &fields); err != nil {
		t.Fatalf("decode project fields: %v", err)
	}
	for _, field := range []string{
		"id",
		"area_id",
		"title",
		"note",
		"done_at",
		"cancelled_at",
		"status",
		"position",
		"created_at",
		"updated_at",
		"stage_id",
		"stage_position",
		"tags",
	} {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON fields = %v, missing %q", fields, field)
		}
	}
	if len(fields) != 13 || string(fields["area_id"]) != "null" || string(fields["done_at"]) != "null" ||
		string(fields["cancelled_at"]) != "null" || string(fields["stage_id"]) != "null" ||
		string(fields["stage_position"]) != "null" || string(fields["tags"]) != "[]" {
		t.Errorf("JSON fields = %v, want complete project row with null exits and tags", fields)
	}
	if addResult.openPath != "chosen.db" || addResult.opens != 1 || addResult.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want chosen path and one open/close", addResult)
	}

	humanAdd := runProjectCommand(
		t,
		&fakeProjectApplication{addResult: created},
		"projects",
		"add",
		"Kitchen reno",
	)
	if humanAdd.exitCode != 0 || humanAdd.stderr != "" ||
		!strings.Contains(humanFields(humanAdd.stdout), "Added project 7: Kitchen reno") {
		t.Errorf("human add result = %#v, want project add narration", humanAdd)
	}

	title := "Bathroom"
	editApplication := &fakeProjectApplication{editResult: project.Edition{Project: project.Project{ID: 7, Title: title}}}
	editResult := runProjectCommand(
		t,
		editApplication,
		"project",
		"edit",
		"7",
		"--title",
		title,
		"--note",
		"",
	)
	if editResult.exitCode != 0 || editResult.stderr != "" {
		t.Fatalf("edit result = %#v, want success", editResult)
	}
	if !strings.Contains(humanFields(editResult.stdout), "Edited: project 7 Bathroom") {
		t.Errorf("stdout = %q, want project edit narration", editResult.stdout)
	}
	if editApplication.editID != 7 || editApplication.editFields.Title == nil ||
		*editApplication.editFields.Title != title || editApplication.editFields.Note == nil ||
		*editApplication.editFields.Note != "" {
		t.Errorf("Edit() ID/fields = %d/%#v, want exact changed fields", editApplication.editID, editApplication.editFields)
	}
}

func TestProjectsAddAccumulatesTagFlagsWithoutSplittingCommas(t *testing.T) {
	t.Parallel()

	created := project.Project{ID: 7, Title: "Kitchen", Tags: []string{"Errands", "home,soon"}}
	application := &fakeProjectApplication{addResult: created}
	result := runProjectCommand(
		t,
		application,
		"projects",
		"add",
		"Kitchen",
		"--tag",
		"Errands",
		"--tag",
		"home,soon",
		"--json",
	)
	requireProjectCommandJSON(t, result, created)
	want := project.AddRequest{Title: "Kitchen", Tags: []string{"Errands", "home,soon"}}
	if !reflect.DeepEqual(application.addFields, want) {
		t.Errorf("Add() fields = %#v, want %#v", application.addFields, want)
	}
}

func TestProjectTagCommandsAdaptExactNamesAndOutputShapes(t *testing.T) {
	t.Parallel()

	tagged := project.Project{ID: 7, Title: "Kitchen", Tags: []string{"Errands", "home,soon"}}
	tagApplication := &fakeProjectApplication{tagResult: project.Tagging{
		Project:   tagged,
		TagTitles: []string{"Errands", "home,soon"},
	}}
	tagResult := runProjectCommand(
		t,
		tagApplication,
		"project",
		"tag",
		"007",
		"ERRANDS",
		"home,soon",
		"--json",
	)
	requireProjectCommandJSON(t, tagResult, tagged)
	if tagApplication.tagID != 7 || !reflect.DeepEqual(tagApplication.tagNames, []string{"ERRANDS", "home,soon"}) {
		t.Errorf("Tag() arguments = (%d, %#v), want (7, exact names)", tagApplication.tagID, tagApplication.tagNames)
	}

	untagged := project.Project{ID: 8, Title: "Plans", Tags: []string{}}
	untagApplication := &fakeProjectApplication{untagResult: project.Tagging{
		Project:   untagged,
		TagTitles: []string{"Errands", "Home\x1b"},
	}}
	untagResult := runProjectCommand(
		t,
		untagApplication,
		"project",
		"untag",
		"8",
		"errands",
		"HOME",
	)
	requireProjectCommandHumanOutput(t, untagResult, "Untagged: project 8 #Errands #Home\\x1b")
	if untagApplication.untagID != 8 || !reflect.DeepEqual(untagApplication.untagNames, []string{"errands", "HOME"}) {
		t.Errorf(
			"Untag() arguments = (%d, %#v), want (8, exact names)",
			untagApplication.untagID,
			untagApplication.untagNames,
		)
	}
}

func TestProjectTagCommandArityAndIDValidationDoNotOpenApplication(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"project", "tag", "7", "--json"},
		{"project", "untag", "--json"},
	} {
		result := runProjectCommand(t, &fakeProjectApplication{}, args...)
		if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("result = %#v, want usage error without application open", result)
		}
	}

	result := runProjectCommand(t, &fakeProjectApplication{}, "project", "tag", "nope", "Errands", "--json")
	got := decodeProjectCommandError(t, result)
	if result.opens != 0 || got.Code != apperr.InvalidArgument {
		t.Errorf("result/error = %#v/%#v, want invalid_argument without application open", result, got)
	}
}

func TestProjectAreaFlagsAdaptContainmentIntent(t *testing.T) {
	t.Parallel()

	areaID := int64(4)
	addApplication := &fakeProjectApplication{addResult: project.Project{ID: 1, AreaID: &areaID}}
	addResult := runProjectCommand(t, addApplication, "projects", "add", "Kitchen", "--area", "004", "--json")
	if addResult.exitCode != 0 || addApplication.addFields.AreaID == nil || *addApplication.addFields.AreaID != 4 {
		t.Fatalf("add = %#v/%#v, want area 4", addResult, addApplication.addFields)
	}

	listApplication := &fakeProjectApplication{listResult: []project.Project{}}
	listResult := runProjectCommand(t, listApplication, "projects", "list", "--area", "5", "--json")
	if listResult.exitCode != 0 || listApplication.listOptions.AreaID == nil || *listApplication.listOptions.AreaID != 5 {
		t.Fatalf("list = %#v/%#v, want area 5", listResult, listApplication.listOptions)
	}

	editApplication := &fakeProjectApplication{editResult: project.Edition{Project: project.Project{ID: 1}}}
	editResult := runProjectCommand(t, editApplication, "project", "edit", "1", "--area", "6", "--json")
	if editResult.exitCode != 0 || editApplication.editFields.Area.Set == nil ||
		*editApplication.editFields.Area.Set != 6 || editApplication.editFields.Area.Clear {
		t.Fatalf("edit = %#v/%#v, want set area 6", editResult, editApplication.editFields)
	}

	clearApplication := &fakeProjectApplication{editResult: project.Edition{Project: project.Project{ID: 1}}}
	clearResult := runProjectCommand(t, clearApplication, "project", "edit", "1", "--no-area", "--json")
	if clearResult.exitCode != 0 || !clearApplication.editFields.Area.Clear || clearApplication.editFields.Area.Set != nil {
		t.Fatalf("edit = %#v/%#v, want clear area", clearResult, clearApplication.editFields)
	}
}

func TestProjectBoardFlagsAdaptMembershipIntentAndOutput(t *testing.T) {
	t.Parallel()

	boardTitle := "Software"
	stageID := int64(4)
	stagePosition := int64(2)
	created := project.Project{
		ID: 1, Title: "Kitchen", StageID: &stageID, StagePosition: &stagePosition, Tags: domain.TagNames{},
	}
	addApplication := &fakeProjectApplication{addResult: created}
	addResult := runProjectCommand(t, addApplication, "projects", "add", "Kitchen", "--board", boardTitle, "--json")
	requireProjectCommandJSON(t, addResult, created)
	if addApplication.addFields.Board == nil || *addApplication.addFields.Board != boardTitle {
		t.Errorf("Add() board = %#v, want exact board title %q", addApplication.addFields.Board, boardTitle)
	}

	edition := project.Edition{
		Project:       created,
		ClearedDefers: []task.Task{},
		Location:      &project.Location{BoardTitle: "Software", StageTitle: "Research"},
	}
	editApplication := &fakeProjectApplication{editResult: edition}
	editResult := runProjectCommand(t, editApplication, "project", "edit", "1", "--board", "software", "--json")
	requireProjectCommandJSON(t, editResult, project.Edition{
		Project: created, ClearedDefers: []task.Task{},
	})
	if editApplication.editFields.Board.Set == nil || *editApplication.editFields.Board.Set != "software" ||
		editApplication.editFields.Board.Clear {
		t.Errorf("Edit() board change = %#v, want set software", editApplication.editFields.Board)
	}

	human := runProjectCommand(t, &fakeProjectApplication{editResult: edition},
		"project", "edit", "1", "--board", "software")
	if human.exitCode != 0 || human.stderr != "" ||
		human.stdout != "~ Edited: ◆ 1  Kitchen → Software/Research\n" {
		t.Errorf("human board edit = %#v, want exact destination mutation", human)
	}

	cleared := project.Edition{
		Project:       project.Project{ID: 1, Title: "Kitchen"},
		ClearedDefers: []task.Task{{ID: 9, Title: "Waiting for Review"}},
	}
	clearApplication := &fakeProjectApplication{editResult: cleared}
	clearResult := runProjectCommand(t, clearApplication, "project", "edit", "1", "--no-board")
	wantClearOutput := "~ Edited: ◆ 1  Kitchen → (no board)\n" +
		"  └ Cleared stage defer: 9  Waiting for Review\n"
	if clearResult.exitCode != 0 || clearResult.stderr != "" ||
		clearResult.stdout != wantClearOutput ||
		!clearApplication.editFields.Board.Clear || clearApplication.editFields.Board.Set != nil {
		t.Errorf("clear board result/call = %#v/%#v, want cleared destination", clearResult, clearApplication.editFields.Board)
	}
}

func TestProjectBoardFlagConflictIsUsageWithoutOpeningApplication(t *testing.T) {
	t.Parallel()

	result := runProjectCommand(
		t,
		&fakeProjectApplication{},
		"project", "edit", "1", "--board", "software", "--no-board", "--json",
	)
	if result.exitCode != 2 || result.opens != 0 || result.closes != 0 ||
		result.stdout != "" || result.stderr == "" {
		t.Errorf("result = %#v, want stderr-only usage error without application lifecycle", result)
	}
	if strings.HasPrefix(result.stderr, "{") {
		t.Errorf("stderr = %q, want human-readable usage diagnostic", result.stderr)
	}
}

func TestProjectAreaGrammarAndValidationHappenBeforeApplicationOpen(t *testing.T) {
	t.Parallel()

	invalid := runProjectCommand(t, &fakeProjectApplication{}, "projects", "add", "Kitchen", "--area", "0", "--json")
	if invalid.exitCode != 1 || invalid.opens != 0 || invalid.stdout != "" {
		t.Errorf("invalid result = %#v, want invalid_argument without open", invalid)
	}
	conflict := runProjectCommand(t, &fakeProjectApplication{}, "project", "edit", "1", "--area", "2", "--no-area", "--json")
	if conflict.exitCode != 2 || conflict.opens != 0 || conflict.stdout != "" {
		t.Errorf("conflict result = %#v, want usage error without open", conflict)
	}
}

func TestProjectListAdaptsStatusAndHumanOutput(t *testing.T) {
	t.Parallel()

	projects := []project.Project{
		{ID: 1, Title: "Kitchen reno", Status: "open"},
		{ID: 12, Title: "Bathroom", Status: "done"},
	}
	application := &fakeProjectApplication{listResult: projects}
	result := runProjectCommand(t, application, "projects", "list")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.listOptions != (project.ListOptions{Status: project.ListStatusOpen}) {
		t.Errorf("List() options = %#v, want default open", application.listOptions)
	}
	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
	if len(lines) != 3 ||
		strings.Join(strings.Fields(lines[0]), " ") != "id title status" ||
		strings.Join(strings.Fields(lines[1]), " ") != "1 Kitchen reno open" ||
		strings.Join(strings.Fields(lines[2]), " ") != "12 Bathroom done" {
		t.Errorf("stdout = %q, want headed ID/title/status rows", result.stdout)
	}

	allApplication := &fakeProjectApplication{listResult: []project.Project{}}
	allResult := runProjectCommand(t, allApplication, "projects", "list", "--status", "all", "--json")
	if allResult.exitCode != 0 || allResult.stderr != "" || allResult.stdout != "[]\n" {
		t.Fatalf("all result = %#v, want empty JSON success", allResult)
	}
	if allApplication.listOptions != (project.ListOptions{Status: project.ListStatusAll}) {
		t.Errorf("List() options = %#v, want all", allApplication.listOptions)
	}
}

func TestProjectShowUsesSchemaOrderFieldValueTable(t *testing.T) {
	t.Parallel()

	doneAt := "2026-07-28T12:00:00.000Z"
	areaID := int64(3)
	shown := project.Project{
		ID:        7,
		AreaID:    &areaID,
		Title:     "Kitchen reno",
		Note:      "Budget: 20k",
		DoneAt:    &doneAt,
		Status:    "done",
		Position:  2,
		CreatedAt: "2026-07-27T12:00:00.000Z",
		UpdatedAt: doneAt,
	}
	application := &fakeProjectApplication{showResult: project.Detail{Project: shown}}
	result := runProjectCommand(t, application, "project", "show", "7")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.showID != 7 {
		t.Errorf("Show() ID = %d, want 7", application.showID)
	}

	wantRows := []string{
		"area 3",
		"board",
		"note Budget: 20k",
		"done at 2026-07-28T12:00:00.000Z",
		"cancelled at",
		"status done",
		"position 2",
		"created at 2026-07-27T12:00:00.000Z",
		"updated at 2026-07-28T12:00:00.000Z",
		"tags",
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) != len(wantRows)+1 {
		t.Fatalf("stdout lines = %q, want %d headline/outline rows", result.stdout, len(wantRows)+1)
	}
	if !strings.HasSuffix(humanFields(lines[0]), "7 Kitchen reno") {
		t.Errorf("headline = %q, want ID and title", lines[0])
	}
	for index, want := range wantRows {
		if got := humanFields(lines[index+1]); got != want {
			t.Errorf("row %d = %q, want %q", index+1, got, want)
		}
	}
	if strings.Contains(result.stdout, "\x1b[") {
		t.Errorf("stdout = %q, want plain output", result.stdout)
	}
}

func TestProjectShowWritesBoardLocationAndBareProjectJSON(t *testing.T) {
	t.Parallel()

	stageID := int64(3)
	stagePosition := int64(1)
	shown := project.Project{
		ID: 7, Title: "Milestone", Status: "open", StageID: &stageID,
		StagePosition: &stagePosition, Tags: domain.TagNames{},
	}
	detail := project.Detail{
		Project:  shown,
		Location: &project.Location{BoardTitle: "Soft\x1bware", StageTitle: "Doing\nnow"},
	}
	human := runProjectCommand(t, &fakeProjectApplication{showResult: detail}, "project", "show", "7")
	if human.exitCode != 0 || human.stderr != "" ||
		!strings.Contains(human.stdout, "    board         Soft\\x1bware/Doing\\nnow\n") {
		t.Errorf("human show = %#v, want escaped board/stage row", human)
	}

	jsonResult := runProjectCommand(t, &fakeProjectApplication{showResult: detail},
		"project", "show", "7", "--json")
	requireProjectCommandJSON(t, jsonResult, shown)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(jsonResult.stdout), &fields); err != nil {
		t.Fatalf("decode shown project: %v", err)
	}
	if _, exists := fields["location"]; exists {
		t.Errorf("JSON fields = %v, want bare project row without display location", fields)
	}
}

func TestProjectResolveCommandsAdaptExitAndJSONEnvelope(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		status string
		exit   project.Exit
	}{
		{name: "done", status: "done", exit: project.ExitDone},
		{name: "cancel", status: "cancelled", exit: project.ExitCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			want := project.Resolution{
				Project: project.Project{
					ID: 7, Title: "Kitchen", Status: test.status, Tags: domain.TagNames{},
				},
				CancelledTasks: []task.Task{},
			}
			application := &fakeProjectApplication{resolveResult: want}
			result := runProjectCommand(t, application, "project", test.name, "007", "--json")
			requireProjectCommandJSON(t, result, want)
			if application.resolveID != 7 || application.resolveExit != test.exit {
				t.Errorf(
					"Resolve() arguments = (%d, %q), want (7, %q)",
					application.resolveID,
					application.resolveExit,
					test.exit,
				)
			}
			if result.opens != 1 || result.closes != 1 {
				t.Errorf("factory lifecycle = %#v, want one open and close", result)
			}
		})
	}
}

func TestProjectMoveAdaptsOptionalPlacementAndWritesOwnedOutputs(t *testing.T) {
	t.Parallel()

	wantProject := project.Project{ID: 7, Title: "Milestone", Status: "open", Tags: domain.TagNames{}}
	tests := []struct {
		name string
		args []string
		want *domain.Placement
	}{
		{name: "bare", args: nil},
		{name: "after", args: []string{"--after=008"}, want: &domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 8}},
		{name: "before", args: []string{"--before=008"}, want: &domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 8}},
		{name: "first", args: []string{"--first"}, want: &domain.Placement{Anchor: domain.PlacementFirst}},
		{name: "last", args: []string{"--last"}, want: &domain.Placement{Anchor: domain.PlacementLast}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeProjectApplication{moveResult: project.Movement{Project: wantProject, StageTitle: "Doing"}}
			args := []string{"project", "move", "007", "doing"}
			args = append(args, test.args...)
			args = append(args, "--json")
			result := runProjectCommand(t, application, args...)
			requireProjectCommandJSON(t, result, wantProject)
			if application.moveID != 7 || application.moveStage != "doing" ||
				!reflect.DeepEqual(application.movePlacement, test.want) {
				t.Errorf("Move() input = (%d, %q, %#v), want (7, doing, %#v)",
					application.moveID, application.moveStage, application.movePlacement, test.want)
			}
			if result.opens != 1 || result.closes != 1 {
				t.Errorf("factory lifecycle = %#v, want one open and close", result)
			}
		})
	}

	human := runProjectCommand(t, &fakeProjectApplication{moveResult: project.Movement{
		Project: project.Project{ID: 7, Title: "Milestone\x1b"}, StageTitle: "Doing\nnow",
	}}, "project", "move", "7", "doing")
	if human.exitCode != 0 || human.stderr != "" ||
		human.stdout != "~ Moved: ◆ 7  Milestone\\x1b → Doing\\nnow\n" {
		t.Errorf("human move = %#v, want exact escaped movement echo", human)
	}
}

func TestProjectMoveGrammarFailuresDoNotOpenApplication(t *testing.T) {
	t.Parallel()

	help := runProjectCommand(t, &fakeProjectApplication{}, "project", "move", "--help")
	if help.exitCode != 0 || help.opens != 0 || help.closes != 0 || help.stderr != "" || help.stdout == "" {
		t.Errorf("move help = %#v, want stdout help without application lifecycle", help)
	}

	for _, test := range []struct {
		args     []string
		wantExit int
	}{
		{args: []string{"project", "move", "7"}, wantExit: 2},
		{args: []string{"project", "move", "7", "doing", "extra"}, wantExit: 2},
		{args: []string{"project", "move", "7", "doing", "--first", "--last"}, wantExit: 2},
		{args: []string{"project", "move", "7", "doing", "--first=false"}, wantExit: 2},
		{args: []string{"project", "move", "nope", "doing"}, wantExit: 1},
		{args: []string{"project", "move", "7", "doing", "--before", "nope"}, wantExit: 1},
	} {
		result := runProjectCommand(t, &fakeProjectApplication{}, test.args...)
		if result.opens != 0 || result.closes != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("move %v = %#v, want pre-open grammar/argument failure", test.args, result)
		}
		if result.exitCode != test.wantExit {
			t.Errorf("move %v exit = %d, want %d", test.args, result.exitCode, test.wantExit)
		}
	}
}

func TestProjectReorderAdaptsEveryPlacementAndWritesBareJSON(t *testing.T) {
	t.Parallel()

	wantProject := project.Project{
		ID: 7, Title: "Kitchen", Status: "open", Position: 3, Tags: domain.TagNames{},
	}
	for _, test := range []struct {
		name string
		flag string
		want domain.Placement
	}{
		{name: "after", flag: "--after=008", want: domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 8}},
		{name: "before", flag: "--before=008", want: domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 8}},
		{name: "first", flag: "--first", want: domain.Placement{Anchor: domain.PlacementFirst}},
		{name: "last", flag: "--last", want: domain.Placement{Anchor: domain.PlacementLast}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeProjectApplication{reorderResult: wantProject}
			result := runProjectCommand(t, application, "project", "reorder", "007", test.flag, "--json")
			requireProjectCommandJSON(t, result, wantProject)
			if application.reorderID != 7 || !reflect.DeepEqual(application.reorderPlacement, test.want) {
				t.Errorf("Reorder() input = (%d, %#v), want (7, %#v)", application.reorderID, application.reorderPlacement, test.want)
			}
			if result.opens != 1 || result.closes != 1 {
				t.Errorf("factory lifecycle = %#v, want one open and close", result)
			}
		})
	}
}

func TestProjectReorderWritesExactHumanMutation(t *testing.T) {
	t.Parallel()

	result := runProjectCommand(
		t,
		&fakeProjectApplication{reorderResult: project.Project{ID: 7, Title: "Kitchen"}},
		"project", "reorder", "7", "--first",
	)
	if result.exitCode != 0 || result.stdout != "~ Reordered: project 7  Kitchen\n" || result.stderr != "" {
		t.Errorf("result = %#v, want exact stdout-only reorder mutation", result)
	}
}

func TestProjectReorderApplicationErrorUsesOnlyStderr(t *testing.T) {
	t.Parallel()

	application := &fakeProjectApplication{reorderError: apperr.New(apperr.NotFound, "no project 99", nil)}
	result := runProjectCommand(t, application, "project", "reorder", "99", "--last", "--json")
	got := decodeProjectCommandError(t, result)
	if got.Code != apperr.NotFound || got.Message != "no project 99" {
		t.Errorf("error = %#v, want project not_found diagnostic", got)
	}
	if result.exitCode != 1 || result.stdout != "" || result.opens != 1 || result.closes != 1 {
		t.Errorf("result = %#v, want stderr-only application failure and one factory lifecycle", result)
	}
}

func TestProjectReopenAndDeleteAdaptArgumentsAndJSONShapes(t *testing.T) {
	t.Parallel()

	reopened := project.Project{
		ID: 8, Title: "Reset", Status: "open", Tags: domain.TagNames{},
	}
	reopenApplication := &fakeProjectApplication{reopenResult: reopened}
	reopenResult := runProjectCommand(t, reopenApplication, "project", "reopen", "8", "--json")
	requireProjectCommandJSON(t, reopenResult, reopened)
	if reopenApplication.reopenID != 8 || reopenResult.opens != 1 || reopenResult.closes != 1 {
		t.Errorf("reopen adaptation/lifecycle = %#v/%#v, want ID 8 and one open/close", reopenApplication, reopenResult)
	}

	deleted := project.Project{
		ID: 9, Title: "Doomed", Status: "open", Tags: domain.TagNames{},
	}
	nonrecursiveApplication := &fakeProjectApplication{deleteResult: project.Deletion{
		Project:      deleted,
		DeletedTasks: []task.Task{},
	}}
	nonrecursiveResult := runProjectCommand(
		t,
		nonrecursiveApplication,
		"project",
		"delete",
		"9",
		"--json",
	)
	requireProjectCommandJSON(t, nonrecursiveResult, deleted)
	if nonrecursiveApplication.deleteID != 9 || nonrecursiveApplication.deleteRecursive {
		t.Errorf(
			"Delete() arguments = (%d, %t), want (9, false)",
			nonrecursiveApplication.deleteID,
			nonrecursiveApplication.deleteRecursive,
		)
	}

	recursiveApplication := &fakeProjectApplication{deleteResult: project.Deletion{
		Project:      deleted,
		DeletedTasks: []task.Task{},
	}}
	recursiveResult := runProjectCommand(
		t,
		recursiveApplication,
		"project",
		"delete",
		"9",
		"--recursive",
		"--json",
	)
	wantDeletion := project.Deletion{Project: deleted, DeletedTasks: []task.Task{}}
	requireProjectCommandJSON(t, recursiveResult, wantDeletion)
	if recursiveApplication.deleteID != 9 || !recursiveApplication.deleteRecursive ||
		recursiveResult.opens != 1 || recursiveResult.closes != 1 {
		t.Errorf(
			"recursive adaptation/lifecycle = (%d, %t)/%#v, want (9, true) and one open/close",
			recursiveApplication.deleteID,
			recursiveApplication.deleteRecursive,
			recursiveResult,
		)
	}
}

func TestProjectLifecycleHumanNarration(t *testing.T) {
	t.Parallel()

	doneResult := runProjectCommand(
		t,
		&fakeProjectApplication{resolveResult: project.Resolution{
			Project: project.Project{ID: 7, Title: "Kitchen\x1b[31m"},
			CancelledTasks: []task.Task{
				{ID: 3, Title: "Pick\rtiles\nnow"},
			},
		}},
		"project",
		"done",
		"7",
	)
	requireProjectCommandHumanOutput(
		t,
		doneResult,
		"Done: project 7 Kitchen\\x1b[31m",
		"Cancelled 1 open task:",
		"3 Pick\\rtiles\\nnow",
	)
	if strings.ContainsAny(doneResult.stdout, "\x1b\r") {
		t.Errorf("done stdout = %q, want terminal controls escaped", doneResult.stdout)
	}

	cancelResult := runProjectCommand(
		t,
		&fakeProjectApplication{resolveResult: project.Resolution{
			Project: project.Project{ID: 8, Title: "Abandon"},
			CancelledTasks: []task.Task{
				{ID: 4, Title: "First"},
				{ID: 3, Title: "Second"},
			},
		}},
		"project",
		"cancel",
		"8",
	)
	requireProjectCommandHumanOutput(
		t,
		cancelResult,
		"Cancelled: project 8 Abandon",
		"Cancelled 2 open tasks:",
		"4 First",
		"3 Second",
	)

	zeroResult := runProjectCommand(
		t,
		&fakeProjectApplication{resolveResult: project.Resolution{
			Project:        project.Project{ID: 10, Title: "Already clear"},
			CancelledTasks: []task.Task{},
		}},
		"project",
		"done",
		"10",
	)
	requireProjectCommandHumanOutput(t, zeroResult, "Done: project 10 Already clear")
	if strings.Count(zeroResult.stdout, "\n") != 1 {
		t.Errorf("zero-cascade stdout = %q, want mutation line only", zeroResult.stdout)
	}

	reopenResult := runProjectCommand(
		t,
		&fakeProjectApplication{reopenResult: project.Project{ID: 10, Title: "Again"}},
		"project",
		"reopen",
		"10",
	)
	requireProjectCommandHumanOutput(t, reopenResult, "Reopened: project 10 Again")
}

func TestProjectRecursiveDeleteHumanNarration(t *testing.T) {
	t.Parallel()

	result := runProjectCommand(
		t,
		&fakeProjectApplication{deleteResult: project.Deletion{
			Project: project.Project{ID: 9, Title: "Doomed"},
			DeletedTasks: []task.Task{
				{ID: 1, Title: "First"},
				{ID: 2, Title: "Second"},
			},
		}},
		"project",
		"delete",
		"9",
		"--recursive",
	)
	requireProjectCommandHumanOutput(
		t,
		result,
		"Deleted: project 9 Doomed",
		"Deleted 2 tasks:",
		"1 First",
		"2 Second",
	)

	emptyResult := runProjectCommand(
		t,
		&fakeProjectApplication{deleteResult: project.Deletion{
			Project:      project.Project{ID: 10, Title: "Empty"},
			DeletedTasks: []task.Task{},
		}},
		"project",
		"delete",
		"10",
		"--recursive",
	)
	requireProjectCommandHumanOutput(t, emptyResult, "Deleted: project 10 Empty")
	if strings.Count(emptyResult.stdout, "\n") != 1 {
		t.Errorf("empty recursive deletion stdout = %q, want mutation line only", emptyResult.stdout)
	}
}

func TestProjectEditWithoutFieldsFailsBeforeOpeningApplication(t *testing.T) {
	t.Parallel()

	result := runProjectCommand(t, &fakeProjectApplication{}, "project", "edit", "7", "--json")
	got := decodeProjectCommandError(t, result)
	if result.opens != 0 {
		t.Errorf("opens = %d, want flag validation before application open", result.opens)
	}
	if got.Code != apperr.InvalidArgument || !strings.Contains(got.Message, "--title") {
		t.Errorf("error = %#v, want invalid_argument naming the edit flags", got)
	}
}

func TestProjectDeleteConflictAddsRecursiveGuidance(t *testing.T) {
	t.Parallel()

	application := &fakeProjectApplication{deleteError: apperr.New(
		apperr.Conflict,
		"cannot delete project 9 while it contains tasks",
		nil,
	)}
	result := runProjectCommand(t, application, "project", "delete", "9", "--json")
	got := decodeProjectCommandError(t, result)
	want := "cannot delete project 9 while it contains tasks; " +
		"use --recursive to delete the project and its tasks"
	if got.Code != apperr.Conflict || got.Message != want {
		t.Errorf("error = %#v, want conflict with --recursive recovery guidance", got)
	}
}

func TestProjectLifecycleErrorUsesErrorStreamAndClosesApplication(t *testing.T) {
	t.Parallel()

	application := &fakeProjectApplication{resolveError: apperr.New(
		apperr.Conflict,
		"project 7 is already done",
		nil,
	)}
	result := runProjectCommand(t, application, "project", "done", "7", "--json")
	got := decodeProjectCommandError(t, result)
	if got.Code != apperr.Conflict || got.Message != "project 7 is already done" {
		t.Errorf("error = %#v, want project conflict diagnostic", got)
	}
	if result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want one open and close", result)
	}
}

func TestTaskShowPlacesContainmentAfterID(t *testing.T) {
	t.Parallel()

	projectID := int64(4)
	areaID := int64(5)
	result := runCommand(t, &fakeApplication{showResult: task.Task{
		ID:        7,
		ProjectID: &projectID,
		AreaID:    &areaID,
		Title:     "Get quotes",
		Status:    "open",
	}}, "show", "7")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) < 3 || !strings.HasSuffix(humanFields(lines[0]), "7 Get quotes") ||
		humanFields(lines[1]) != "project 4" ||
		humanFields(lines[2]) != "area 5" {
		t.Errorf("first rows = %q, want headline, project, then area", result.stdout)
	}
}

func TestProjectValidationAndErrorsUseExpectedLifecycle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "invalid show ID", args: []string{"project", "show", "0", "--json"}},
		{name: "invalid done ID", args: []string{"project", "done", "0", "--json"}},
		{name: "invalid cancel ID", args: []string{"project", "cancel", "+1", "--json"}},
		{name: "invalid reopen ID", args: []string{"project", "reopen", "nope", "--json"}},
		{name: "invalid delete ID", args: []string{"--json", "project", "delete", "--", "-1"}},
		{name: "invalid status", args: []string{"projects", "list", "--status", "later", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runProjectCommand(t, &fakeProjectApplication{}, test.args...)
			got := decodeProjectCommandError(t, result)
			if result.opens != 0 {
				t.Errorf("opens = %d, want validation before application open", result.opens)
			}
			if got.Code != apperr.InvalidArgument {
				t.Errorf("error code = %q, want invalid_argument", got.Code)
			}
		})
	}

	application := &fakeProjectApplication{showError: apperr.New(apperr.NotFound, "no project 99", nil)}
	result := runProjectCommand(t, application, "project", "show", "99", "--json")
	got := decodeProjectCommandError(t, result)
	if result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want one open and close", result)
	}
	if got.Code != apperr.NotFound || got.Message != "no project 99" {
		t.Errorf("error = %#v, want project not_found diagnostic", got)
	}
}
