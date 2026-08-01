package cmd

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeProjectApplication struct {
	addResult       project.Project
	addError        error
	listResult      []project.Project
	listError       error
	showResult      project.Project
	showError       error
	editResult      project.Project
	editError       error
	resolveResult   project.Resolution
	resolveError    error
	reopenResult    project.Project
	reopenError     error
	deleteResult    project.Deletion
	deleteError     error
	addFields       project.AddFields
	listOptions     project.ListOptions
	showID          int64
	editID          int64
	editFields      project.EditFields
	resolveID       int64
	resolveExit     project.Exit
	reopenID        int64
	deleteID        int64
	deleteRecursive bool
}

func (f *fakeProjectApplication) Add(
	_ context.Context,
	fields project.AddFields,
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
) (project.Project, error) {
	f.showID = id
	return f.showResult, f.showError
}

func (f *fakeProjectApplication) Edit(
	_ context.Context,
	id int64,
	fields project.EditFields,
) (project.Project, error) {
	f.editID = id
	f.editFields = fields
	return f.editResult, f.editError
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

func requireProjectCommandHumanOutput(t *testing.T, result commandResult, want string) {
	t.Helper()
	if result.exitCode != 0 || result.stderr != "" || result.stdout != want {
		t.Errorf("result = %#v, want stdout %q", result, want)
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
	created := project.Project{ID: 7, Title: "Kitchen reno", Note: note, Status: "open"}
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
	if addApplication.addFields != (project.AddFields{Title: "Kitchen reno", Note: note}) {
		t.Errorf("Add() fields = %#v, want exact title and stdin note", addApplication.addFields)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(addResult.stdout), &fields); err != nil {
		t.Fatalf("decode project fields: %v", err)
	}
	for _, field := range []string{
		"id",
		"title",
		"note",
		"done_at",
		"cancelled_at",
		"status",
		"position",
		"created_at",
		"updated_at",
	} {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON fields = %v, missing %q", fields, field)
		}
	}
	if len(fields) != 9 || string(fields["done_at"]) != "null" || string(fields["cancelled_at"]) != "null" {
		t.Errorf("JSON fields = %v, want complete project row with null exits", fields)
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
		humanAdd.stdout != "Added project 7: Kitchen reno\n" {
		t.Errorf("human add result = %#v, want project add narration", humanAdd)
	}

	title := "Bathroom"
	editApplication := &fakeProjectApplication{editResult: project.Project{ID: 7, Title: title}}
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
	if editResult.stdout != "Edited: project 7  Bathroom\n" {
		t.Errorf("stdout = %q, want project edit narration", editResult.stdout)
	}
	if editApplication.editID != 7 || editApplication.editFields.Title == nil ||
		*editApplication.editFields.Title != title || editApplication.editFields.Note == nil ||
		*editApplication.editFields.Note != "" {
		t.Errorf("Edit() ID/fields = %d/%#v, want exact changed fields", editApplication.editID, editApplication.editFields)
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
	if len(lines) != 2 ||
		strings.Join(strings.Fields(lines[0]), " ") != "1 Kitchen reno open" ||
		strings.Join(strings.Fields(lines[1]), " ") != "12 Bathroom done" {
		t.Errorf("stdout = %q, want headerless ID/title/status rows", result.stdout)
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
	shown := project.Project{
		ID:        7,
		Title:     "Kitchen reno",
		Note:      "Budget: 20k",
		DoneAt:    &doneAt,
		Status:    "done",
		Position:  2,
		CreatedAt: "2026-07-27T12:00:00.000Z",
		UpdatedAt: doneAt,
	}
	application := &fakeProjectApplication{showResult: shown}
	result := runProjectCommand(t, application, "project", "show", "7")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.showID != 7 {
		t.Errorf("Show() ID = %d, want 7", application.showID)
	}

	wantLabels := []string{
		"ID",
		"Title",
		"Note",
		"Done at",
		"Cancelled at",
		"Status",
		"Position",
		"Created at",
		"Updated at",
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) != len(wantLabels) {
		t.Fatalf("stdout lines = %q, want %d schema rows", result.stdout, len(wantLabels))
	}
	for index, label := range wantLabels {
		if !strings.HasPrefix(strings.TrimSpace(lines[index]), label) {
			t.Errorf("row %d = %q, want label %q", index, lines[index], label)
		}
	}
	if strings.Contains(result.stdout, "\x1b[") {
		t.Errorf("stdout = %q, want plain output", result.stdout)
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
				Project:        project.Project{ID: 7, Title: "Kitchen", Status: test.status},
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

func TestProjectReopenAndDeleteAdaptArgumentsAndJSONShapes(t *testing.T) {
	t.Parallel()

	reopened := project.Project{ID: 8, Title: "Reset", Status: "open"}
	reopenApplication := &fakeProjectApplication{reopenResult: reopened}
	reopenResult := runProjectCommand(t, reopenApplication, "project", "reopen", "8", "--json")
	requireProjectCommandJSON(t, reopenResult, reopened)
	if reopenApplication.reopenID != 8 || reopenResult.opens != 1 || reopenResult.closes != 1 {
		t.Errorf("reopen adaptation/lifecycle = %#v/%#v, want ID 8 and one open/close", reopenApplication, reopenResult)
	}

	deleted := project.Project{ID: 9, Title: "Doomed", Status: "open"}
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
	wantDone := "Done: project 7  Kitchen\\x1b[31m\n" +
		"Cancelled 1 open task:\n" +
		"  3  Pick\\rtiles\\nnow\n"
	requireProjectCommandHumanOutput(t, doneResult, wantDone)
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
	wantCancel := "Cancelled: project 8  Abandon\n" +
		"Cancelled 2 open tasks:\n" +
		"  4  First\n" +
		"  3  Second\n"
	requireProjectCommandHumanOutput(t, cancelResult, wantCancel)

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
	requireProjectCommandHumanOutput(t, zeroResult, "Done: project 10  Already clear\n")

	reopenResult := runProjectCommand(
		t,
		&fakeProjectApplication{reopenResult: project.Project{ID: 10, Title: "Again"}},
		"project",
		"reopen",
		"10",
	)
	requireProjectCommandHumanOutput(t, reopenResult, "Reopened: project 10  Again\n")
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
	want := "Deleted: project 9  Doomed\n" +
		"Deleted 2 tasks:\n" +
		"  1  First\n" +
		"  2  Second\n"
	requireProjectCommandHumanOutput(t, result, want)

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
	requireProjectCommandHumanOutput(t, emptyResult, "Deleted: project 10  Empty\n")
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

func TestTaskShowPlacesProjectAfterID(t *testing.T) {
	t.Parallel()

	projectID := int64(4)
	result := runCommand(t, &fakeApplication{showResult: task.Task{
		ID:        7,
		ProjectID: &projectID,
		Title:     "Get quotes",
		Status:    "open",
	}}, "show", "7")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) < 2 || strings.Join(strings.Fields(lines[0]), " ") != "ID 7" ||
		strings.Join(strings.Fields(lines[1]), " ") != "Project 4" {
		t.Errorf("first rows = %q, want ID then Project", result.stdout)
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
