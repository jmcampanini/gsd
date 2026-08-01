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
	addResult   project.Project
	addError    error
	listResult  []project.Project
	listError   error
	showResult  project.Project
	showError   error
	editResult  project.Project
	editError   error
	addFields   project.AddFields
	listOptions project.ListOptions
	showID      int64
	editID      int64
	editFields  project.EditFields
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

func TestTaskProjectFlagUsesDecimalIDGrammarWithoutOpeningDatabase(t *testing.T) {
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
	if addResult.exitCode != 0 || addResult.stderr != "" {
		t.Fatalf("add result = %#v, want success", addResult)
	}
	if addApplication.addFields != (project.AddFields{Title: "Kitchen reno", Note: note}) {
		t.Errorf("Add() fields = %#v, want exact title and stdin note", addApplication.addFields)
	}
	var decoded project.Project
	if err := json.Unmarshal([]byte(addResult.stdout), &decoded); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	if !reflect.DeepEqual(decoded, created) {
		t.Errorf("JSON project = %#v, want %#v", decoded, created)
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
		{name: "invalid ID", args: []string{"project", "show", "0", "--json"}},
		{name: "invalid status", args: []string{"projects", "list", "--status", "later", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runProjectCommand(t, &fakeProjectApplication{}, test.args...)
			if result.exitCode != 1 || result.opens != 0 || result.stdout != "" {
				t.Errorf("result = %#v, want application error without open", result)
			}
			var envelope errorEnvelope
			if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if envelope.Error.Code != apperr.InvalidArgument {
				t.Errorf("error code = %q, want invalid_argument", envelope.Error.Code)
			}
		})
	}

	application := &fakeProjectApplication{showError: apperr.New(apperr.NotFound, "no project 99", nil)}
	result := runProjectCommand(t, application, "project", "show", "99", "--json")
	if result.exitCode != 1 || result.opens != 1 || result.closes != 1 || result.stdout != "" {
		t.Fatalf("result = %#v, want stderr-only not_found with one lifecycle", result)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != apperr.NotFound || envelope.Error.Message != "no project 99" {
		t.Errorf("error = %#v, want project not_found diagnostic", envelope.Error)
	}
}
