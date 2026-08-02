package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeApplication struct {
	addResult       task.Task
	addError        error
	inboxResult     []task.ViewTask
	inboxError      error
	availableResult []task.ViewTask
	availableError  error
	listResult      []task.Task
	listError       error
	showResult      task.Task
	showError       error
	editResult      task.Task
	editError       error
	doneResult      task.Task
	doneError       error
	cancelResult    task.Task
	cancelError     error
	reopenResult    task.Task
	reopenError     error
	deleteResult    task.Task
	deleteError     error
	addTitle        string
	addNote         string
	addProjectID    *int64
	addAreaID       *int64
	addDueOn        *string
	addDeferUntil   *string
	listOptions     task.ListOptions
	showID          int64
	editID          int64
	editFields      task.EditFields
	mutation        string
	mutationID      int64
}

func (f *fakeApplication) Add(_ context.Context, fields task.AddFields) (task.Task, error) {
	f.addTitle = fields.Title
	f.addNote = fields.Note
	f.addProjectID = fields.ProjectID
	f.addAreaID = fields.AreaID
	f.addDueOn = fields.DueOn
	f.addDeferUntil = fields.DeferUntil
	return f.addResult, f.addError
}

func (f *fakeApplication) Inbox(context.Context) ([]task.ViewTask, error) {
	return f.inboxResult, f.inboxError
}

func (f *fakeApplication) Available(context.Context) ([]task.ViewTask, error) {
	return f.availableResult, f.availableError
}

func (f *fakeApplication) List(_ context.Context, options task.ListOptions) ([]task.Task, error) {
	f.listOptions = options
	return f.listResult, f.listError
}

func (f *fakeApplication) Show(_ context.Context, id int64) (task.Task, error) {
	f.showID = id
	return f.showResult, f.showError
}

func (f *fakeApplication) Edit(
	_ context.Context,
	id int64,
	fields task.EditFields,
) (task.Task, error) {
	f.editID = id
	f.editFields = fields
	return f.editResult, f.editError
}

func (f *fakeApplication) Done(_ context.Context, id int64) (task.Task, error) {
	f.mutation = "done"
	f.mutationID = id
	return f.doneResult, f.doneError
}

func (f *fakeApplication) Cancel(_ context.Context, id int64) (task.Task, error) {
	f.mutation = "cancel"
	f.mutationID = id
	return f.cancelResult, f.cancelError
}

func (f *fakeApplication) Reopen(_ context.Context, id int64) (task.Task, error) {
	f.mutation = "reopen"
	f.mutationID = id
	return f.reopenResult, f.reopenError
}

func (f *fakeApplication) Delete(_ context.Context, id int64) (task.Task, error) {
	f.mutation = "delete"
	f.mutationID = id
	return f.deleteResult, f.deleteError
}

type commandResult struct {
	stdout   string
	stderr   string
	exitCode int
	openPath string
	opens    int
	closes   int
}

func runCommand(t *testing.T, application task.Application, args ...string) commandResult {
	t.Helper()
	return runCommandWithInput(t, application, strings.NewReader(""), args...)
}

func runProjectCommand(t *testing.T, application project.Application, args ...string) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{projects: application}, strings.NewReader(""), args...)
}

func runProjectCommandWithInput(
	t *testing.T,
	application project.Application,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{projects: application}, input, args...)
}

func runCommandWithInput(
	t *testing.T,
	application task.Application,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{tasks: application}, input, args...)
}

func runCommandWithApplications(
	t *testing.T,
	available applications,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := commandResult{}
	factory := func(_ context.Context, path string) (applications, io.Closer, error) {
		result.opens++
		result.openPath = path
		return available, closeRecorder{close: func() { result.closes++ }}, nil
	}
	root := newRootCommandWithFactory(factory)
	root.SetIn(input)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	result.exitCode = execute(root, args)
	result.stdout = stdout.String()
	result.stderr = stderr.String()

	return result
}

type closeRecorder struct {
	close func()
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("stdin failed")
}

func (c closeRecorder) Close() error {
	c.close()
	return nil
}

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: 0},
		{
			name: "application",
			err:  apperr.New(apperr.Conflict, "conflict", nil),
			want: 1,
		},
		{
			name: "wrapped application",
			err:  errors.Join(errors.New("context"), apperr.New(apperr.NotFound, "missing", nil)),
			want: 1,
		},
		{name: "usage", err: errors.New("usage error"), want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := exitCodeForError(test.err); got != test.want {
				t.Fatalf("exitCodeForError() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestNormalizeApplicationErrorAddsTypedUnarchiveGuidanceDeterministically(t *testing.T) {
	t.Parallel()

	cause := &area.ArchivedAreasError{IDs: []int64{9, 2, 9, 4}}
	original := apperr.New(apperr.Conflict, "archived areas block this operation", cause)
	normalized := normalizeApplicationError(original)
	code, ok := apperr.CodeOf(normalized)
	if !ok || code != apperr.Conflict {
		t.Fatalf("normalized code = %q/%t, want conflict", code, ok)
	}
	want := "archived areas block this operation; unarchive first: gsd area unarchive 2; gsd area unarchive 4; gsd area unarchive 9"
	if normalized.Error() != want {
		t.Errorf("normalized error = %q, want %q", normalized, want)
	}
	if !errors.Is(normalized, original) || !errors.Is(normalized, cause) {
		t.Errorf("normalized error = %v, want original typed cause preserved", normalized)
	}
}

func TestNormalizeApplicationErrorAddsTypedReopenGuidanceDeterministically(t *testing.T) {
	t.Parallel()

	cause := &project.ResolvedProjectsError{IDs: []int64{9, 2, 9, 4}}
	original := apperr.New(apperr.Conflict, "resolved projects block this operation", cause)
	normalized := normalizeApplicationError(original)
	code, ok := apperr.CodeOf(normalized)
	if !ok || code != apperr.Conflict {
		t.Fatalf("normalized code = %q/%t, want conflict", code, ok)
	}
	want := "resolved projects block this operation; reopen first: gsd project reopen 2; gsd project reopen 4; gsd project reopen 9"
	if normalized.Error() != want {
		t.Errorf("normalized error = %q, want %q", normalized, want)
	}
	if !errors.Is(normalized, original) || !errors.Is(normalized, cause) {
		t.Errorf("normalized error = %v, want original typed cause preserved", normalized)
	}
}

func TestNormalizeApplicationErrorComposesReopenBeforeUnarchiveGuidance(t *testing.T) {
	t.Parallel()

	original := apperr.New(
		apperr.Conflict,
		"cannot move task 3 while project 1 is resolved and area 2 is archived",
		errors.Join(
			&project.ResolvedProjectsError{IDs: []int64{1}},
			&area.ArchivedAreasError{IDs: []int64{2}},
		),
	)
	normalized := normalizeApplicationError(original)
	want := "cannot move task 3 while project 1 is resolved and area 2 is archived" +
		"; reopen first: gsd project reopen 1; unarchive first: gsd area unarchive 2"
	if normalized.Error() != want {
		t.Errorf("normalized error = %q, want %q", normalized, want)
	}
}

func TestTypedArchivedAreaErrorWritesJSONGuidanceToStderr(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{doneError: apperr.New(
		apperr.Conflict,
		"cannot complete task 7 while its governing area is archived",
		&area.ArchivedAreasError{IDs: []int64{3}},
	)}
	result := runCommand(t, application, "done", "7", "--json")
	if result.exitCode != 1 || result.stdout != "" || result.opens != 1 || result.closes != 1 {
		t.Fatalf("result = %#v, want stderr-only conflict and one open/close", result)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode stderr: %v", err)
	}
	want := "cannot complete task 7 while its governing area is archived; unarchive first: gsd area unarchive 3"
	if envelope.Error.Code != apperr.Conflict || envelope.Error.Message != want {
		t.Errorf("error = %#v, want typed unarchive guidance", envelope.Error)
	}
}

func TestJSONCommandOutput(t *testing.T) {
	t.Parallel()

	dueOn := "2026-07-28"
	deferUntil := "2026-07-27"
	created := task.Task{
		ID:         7,
		Title:      "capture",
		Note:       "details",
		DueOn:      &dueOn,
		DeferUntil: &deferUntil,
		Status:     "open",
		Position:   2,
		CreatedAt:  "2026-07-27T12:00:00.000Z",
		UpdatedAt:  "2026-07-27T12:00:00.000Z",
	}
	application := &fakeApplication{addResult: created}
	result := runCommand(
		t,
		application,
		"add",
		"capture",
		"--note",
		"details",
		"--due",
		"tomorrow",
		"--defer",
		"today",
		"--db",
		"chosen.db",
		"--json",
	)

	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
	}
	if result.stderr != "" {
		t.Errorf("stderr = %q, want empty", result.stderr)
	}
	if !strings.HasSuffix(result.stdout, "\n") || strings.Count(result.stdout, "\n") != 1 {
		t.Errorf("stdout = %q, want one newline-terminated JSON value", result.stdout)
	}
	var got task.Task
	if err := json.Unmarshal([]byte(result.stdout), &got); err != nil {
		t.Fatalf("decode stdout: %v", err)
	}
	if !reflect.DeepEqual(got, created) {
		t.Errorf("JSON task = %#v, want %#v", got, created)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &fields); err != nil {
		t.Fatalf("decode JSON fields: %v", err)
	}
	for _, field := range []string{
		"id",
		"project_id",
		"area_id",
		"title",
		"note",
		"defer_until",
		"due_on",
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
	if len(fields) != 13 {
		t.Errorf("JSON field count = %d, want 13", len(fields))
	}
	if string(fields["project_id"]) != "null" || string(fields["area_id"]) != "null" ||
		string(fields["done_at"]) != "null" || string(fields["cancelled_at"]) != "null" {
		t.Errorf(
			"nullable fields = (%s, %s, %s, %s), want null",
			fields["project_id"],
			fields["area_id"],
			fields["done_at"],
			fields["cancelled_at"],
		)
	}
	if application.addTitle != "capture" || application.addNote != "details" ||
		application.addDueOn == nil || *application.addDueOn != "tomorrow" ||
		application.addDeferUntil == nil || *application.addDeferUntil != "today" {
		t.Errorf(
			"Add() input = (%q, %q, %#v, %#v), want exact command input",
			application.addTitle,
			application.addNote,
			application.addDueOn,
			application.addDeferUntil,
		)
	}
	if result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want chosen path and one open/close", result)
	}
}

func TestAddAndEditReadNotesFromStdinExactly(t *testing.T) {
	t.Parallel()

	note := "line one\nline two\n"
	addApplication := &fakeApplication{addResult: task.Task{ID: 1, Title: "capture", Note: note}}
	addResult := runCommandWithInput(
		t,
		addApplication,
		strings.NewReader(note),
		"add",
		"capture",
		"--note",
		"-",
		"--json",
	)
	if addResult.exitCode != 0 || addResult.stderr != "" {
		t.Fatalf("add result = %#v, want success", addResult)
	}
	if addApplication.addNote != note {
		t.Errorf("Add() note = %q, want exact stdin %q", addApplication.addNote, note)
	}

	editApplication := &fakeApplication{editResult: task.Task{ID: 7, Title: "capture", Note: note}}
	editResult := runCommandWithInput(
		t,
		editApplication,
		strings.NewReader(note),
		"edit",
		"7",
		"--note",
		"-",
		"--json",
	)
	if editResult.exitCode != 0 || editResult.stderr != "" {
		t.Fatalf("edit result = %#v, want success", editResult)
	}
	if editApplication.editID != 7 || editApplication.editFields.Title != nil {
		t.Errorf("Edit() ID/title = %d/%#v, want 7/omitted", editApplication.editID, editApplication.editFields.Title)
	}
	if editApplication.editFields.Note == nil || *editApplication.editFields.Note != note {
		t.Errorf("Edit() note = %#v, want exact stdin %q", editApplication.editFields.Note, note)
	}
}

func TestNoteStdinReadFailureIsInternalWithoutOpeningApplication(t *testing.T) {
	t.Parallel()

	result := runCommandWithInput(
		t,
		&fakeApplication{},
		failingReader{},
		"add",
		"capture",
		"--note",
		"-",
		"--json",
	)
	if result.exitCode != 1 || result.stdout != "" || result.opens != 0 {
		t.Errorf("result = %#v, want stderr-only internal error without application open", result)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != apperr.Internal {
		t.Errorf("error code = %q, want internal", envelope.Error.Code)
	}
}

func TestEditAdaptsFieldsAndHumanOutput(t *testing.T) {
	t.Parallel()

	edited := task.Task{ID: 7, Title: "  revised  ", Status: "open"}
	application := &fakeApplication{editResult: edited}
	result := runCommand(
		t,
		application,
		"edit",
		"7",
		"--title",
		"  revised  ",
		"--note",
		"",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if result.stdout != "Edited: 7    revised  \n" {
		t.Errorf("stdout = %q, want edited task", result.stdout)
	}
	if application.editFields.Title == nil || *application.editFields.Title != "  revised  " {
		t.Errorf("Edit() title = %#v, want exact requested title", application.editFields.Title)
	}
	if application.editFields.Note == nil || *application.editFields.Note != "" {
		t.Errorf("Edit() note = %#v, want explicit empty string", application.editFields.Note)
	}
	if application.editFields.DueOn != (task.DateChange{}) {
		t.Errorf("Edit() due change = %#v, want omitted", application.editFields.DueOn)
	}
}

func TestEditAdaptsDueIntent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, task.DateChange)
	}{
		{
			name: "set",
			args: []string{"--due", "+1d"},
			check: func(t *testing.T, change task.DateChange) {
				t.Helper()
				if change.Set == nil || *change.Set != "+1d" || change.Clear {
					t.Errorf("due change = %#v, want set +1d", change)
				}
			},
		},
		{
			name: "clear",
			args: []string{"--no-due"},
			check: func(t *testing.T, change task.DateChange) {
				t.Helper()
				if !change.Clear || change.Set != nil {
					t.Errorf("due change = %#v, want explicit clear", change)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeApplication{editResult: task.Task{ID: 7, Title: "capture"}}
			args := append([]string{"edit", "7"}, test.args...)
			args = append(args, "--json")
			result := runCommand(t, application, args...)
			if result.exitCode != 0 || result.stderr != "" {
				t.Fatalf("result = %#v, want success", result)
			}
			test.check(t, application.editFields.DueOn)
		})
	}
}

func TestEditAdaptsDeferIntent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, task.DateChange)
	}{
		{
			name: "set",
			args: []string{"--defer", "+1d"},
			check: func(t *testing.T, change task.DateChange) {
				t.Helper()
				if change.Set == nil || *change.Set != "+1d" || change.Clear {
					t.Errorf("defer change = %#v, want set +1d", change)
				}
			},
		},
		{
			name: "clear",
			args: []string{"--no-defer"},
			check: func(t *testing.T, change task.DateChange) {
				t.Helper()
				if !change.Clear || change.Set != nil {
					t.Errorf("defer change = %#v, want explicit clear", change)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeApplication{editResult: task.Task{ID: 7, Title: "capture"}}
			args := append([]string{"edit", "7"}, test.args...)
			args = append(args, "--json")
			result := runCommand(t, application, args...)
			if result.exitCode != 0 || result.stderr != "" {
				t.Fatalf("result = %#v, want success", result)
			}
			test.check(t, application.editFields.DeferUntil)
		})
	}
}

func TestEditWithoutFieldsFailsBeforeOpeningApplication(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{}, "edit", "7", "--json")
	if result.exitCode != 1 || result.stdout != "" || result.opens != 0 {
		t.Errorf("result = %#v, want stderr-only error before opening the application", result)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != apperr.InvalidArgument ||
		!strings.Contains(envelope.Error.Message, "--title") {
		t.Errorf("error = %#v, want invalid_argument naming the edit flags", envelope.Error)
	}
}

func TestEmptyInboxJSONIsArray(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{inboxResult: []task.ViewTask{}}, "inbox", "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
	}
	if result.stdout != "[]\n" {
		t.Errorf("stdout = %q, want empty JSON array", result.stdout)
	}
}

func TestAvailableAdaptsOutputModes(t *testing.T) {
	t.Parallel()

	deferUntil := "2026-07-28"
	tasks := []task.ViewTask{{Task: task.Task{ID: 7, Title: "actionable", DeferUntil: &deferUntil, Status: "open"}}}
	jsonResult := runCommand(t, &fakeApplication{availableResult: tasks}, "available", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stderr != "" {
		t.Fatalf("JSON available result = %#v, want success", jsonResult)
	}
	var decoded []task.ViewTask
	if err := json.Unmarshal([]byte(jsonResult.stdout), &decoded); err != nil {
		t.Fatalf("decode available JSON: %v", err)
	}
	if !reflect.DeepEqual(decoded, tasks) {
		t.Errorf("available JSON = %#v, want %#v", decoded, tasks)
	}

	humanResult := runCommand(t, &fakeApplication{availableResult: tasks}, "available")
	if humanResult.exitCode != 0 || humanResult.stderr != "" {
		t.Fatalf("human available result = %#v, want success", humanResult)
	}
	if strings.Join(strings.Fields(humanResult.stdout), " ") != "7 actionable defer 2026-07-28" {
		t.Errorf("human available = %q, want inbox-style row without status", humanResult.stdout)
	}

	emptyResult := runCommand(t, &fakeApplication{availableResult: []task.ViewTask{}}, "available", "--json")
	if emptyResult.exitCode != 0 || emptyResult.stdout != "[]\n" || emptyResult.stderr != "" {
		t.Errorf("empty available = %#v, want empty JSON array", emptyResult)
	}
}

func TestInboxJSONIncludesExactViewTaskEnrichment(t *testing.T) {
	t.Parallel()

	projectID := int64(2)
	areaID := int64(3)
	projectTitle := "Kitchen"
	areaTitle := "Home"
	view := task.ViewTask{
		Task:               task.Task{ID: 1, ProjectID: &projectID, Title: "Get quotes", Status: "open"},
		ProjectTitle:       &projectTitle,
		GoverningAreaID:    &areaID,
		GoverningAreaTitle: &areaTitle,
	}
	result := runCommand(t, &fakeApplication{inboxResult: []task.ViewTask{view}}, "inbox", "--json")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	var fields []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &fields); err != nil {
		t.Fatalf("decode inbox JSON: %v", err)
	}
	want := []string{
		"id", "project_id", "area_id", "title", "note", "defer_until", "due_on", "done_at",
		"cancelled_at", "status", "position", "created_at", "updated_at", "project_title",
		"governing_area_id", "governing_area_title",
	}
	if len(fields) != 1 || len(fields[0]) != len(want) {
		t.Fatalf("fields = %v, want exact view row", fields)
	}
	for _, name := range want {
		if _, ok := fields[0][name]; !ok {
			t.Errorf("fields = %v, missing %q", fields[0], name)
		}
	}
	if string(fields[0]["project_title"]) != `"Kitchen"` ||
		string(fields[0]["governing_area_id"]) != "3" ||
		string(fields[0]["governing_area_title"]) != `"Home"` {
		t.Errorf("enrichment = %v, want project and governing area", fields[0])
	}
}

func TestListAdaptsStatusAndOutputMode(t *testing.T) {
	t.Parallel()

	tasks := []task.Task{
		{ID: 1, Title: "first", Status: "done"},
		{ID: 2, Title: "second", Status: "cancelled"},
	}
	jsonApplication := &fakeApplication{listResult: tasks}
	jsonResult := runCommand(t, jsonApplication, "list", "--status", "all", "--json")
	if jsonResult.exitCode != 0 || jsonResult.stderr != "" {
		t.Fatalf("JSON list result = %#v, want success", jsonResult)
	}
	var decoded []task.Task
	if err := json.Unmarshal([]byte(jsonResult.stdout), &decoded); err != nil {
		t.Fatalf("decode list JSON: %v", err)
	}
	if len(decoded) != len(tasks) || decoded[0] != tasks[0] || decoded[1] != tasks[1] {
		t.Errorf("list JSON = %#v, want %#v", decoded, tasks)
	}
	if jsonApplication.listOptions != (task.ListOptions{Status: task.ListStatusAll}) {
		t.Errorf("list options = %#v, want all without date selector", jsonApplication.listOptions)
	}

	humanApplication := &fakeApplication{listResult: tasks}
	humanResult := runCommand(t, humanApplication, "list")
	if humanResult.exitCode != 0 || humanResult.stderr != "" {
		t.Fatalf("human list result = %#v, want success", humanResult)
	}
	if humanApplication.listOptions != (task.ListOptions{Status: task.ListStatusOpen}) {
		t.Errorf("default list options = %#v, want open without date selector", humanApplication.listOptions)
	}
	lines := strings.Split(strings.TrimSpace(humanResult.stdout), "\n")
	if len(lines) != 2 ||
		strings.Join(strings.Fields(lines[0]), " ") != "1 first done" ||
		strings.Join(strings.Fields(lines[1]), " ") != "2 second cancelled" {
		t.Errorf("human list = %q, want distinct ID, title, and status columns", humanResult.stdout)
	}

	emptyResult := runCommand(t, &fakeApplication{listResult: []task.Task{}}, "list")
	if emptyResult.exitCode != 0 || emptyResult.stdout != "" || emptyResult.stderr != "" {
		t.Errorf("empty human list = %#v, want no output", emptyResult)
	}
}

func TestListAdaptsDeadlineSelectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		flag     string
		selector task.DateSelector
	}{
		{name: "due", flag: "--due", selector: task.DateSelectorDue},
		{name: "overdue", flag: "--overdue", selector: task.DateSelectorOverdue},
		{name: "deferred", flag: "--deferred", selector: task.DateSelectorDeferred},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeApplication{listResult: []task.Task{}}
			result := runCommand(t, application, "list", "--status", "all", test.flag, "--json")
			if result.exitCode != 0 || result.stdout != "[]\n" || result.stderr != "" {
				t.Fatalf("result = %#v, want empty JSON success", result)
			}
			want := task.ListOptions{Status: task.ListStatusAll, Date: test.selector}
			if application.listOptions != want {
				t.Errorf("list options = %#v, want %#v", application.listOptions, want)
			}
		})
	}
}

func TestDateFlagConflictsAreUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "edit due", args: []string{"edit", "7", "--due", "today", "--no-due", "--json"}},
		{name: "edit defer", args: []string{"edit", "7", "--defer", "today", "--no-defer", "--json"}},
		{name: "list due overdue", args: []string{"list", "--due", "--overdue", "--json"}},
		{name: "list due deferred", args: []string{"list", "--due", "--deferred", "--json"}},
		{name: "list overdue deferred", args: []string{"list", "--overdue", "--deferred", "--json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, &fakeApplication{}, test.args...)
			if result.exitCode != 2 || result.opens != 0 || result.stdout != "" {
				t.Errorf("result = %#v, want stderr-only usage error without open", result)
			}
			if result.stderr == "" || strings.HasPrefix(result.stderr, "{") {
				t.Errorf("stderr = %q, want human-readable usage diagnostic", result.stderr)
			}
		})
	}
}

func TestLifecycleCommandsAdaptIDsAndHumanActions(t *testing.T) {
	t.Parallel()

	affected := task.Task{ID: 7, Title: "ship it", Status: "done"}
	tests := []struct {
		command     string
		action      string
		application *fakeApplication
	}{
		{command: "done", action: "Done", application: &fakeApplication{doneResult: affected}},
		{command: "cancel", action: "Cancelled", application: &fakeApplication{cancelResult: affected}},
		{command: "reopen", action: "Reopened", application: &fakeApplication{reopenResult: affected}},
		{command: "delete", action: "Deleted", application: &fakeApplication{deleteResult: affected}},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, test.application, test.command, "7")
			if result.exitCode != 0 || result.stderr != "" {
				t.Fatalf("result = %#v, want success", result)
			}
			if result.stdout != test.action+": 7  ship it\n" {
				t.Errorf("stdout = %q, want action and affected task", result.stdout)
			}
			if test.application.mutation != test.command || test.application.mutationID != 7 {
				t.Errorf(
					"mutation call = (%q, %d), want (%q, 7)",
					test.application.mutation,
					test.application.mutationID,
					test.command,
				)
			}
		})
	}
}

func TestLifecycleJSONReturnsAffectedTask(t *testing.T) {
	t.Parallel()

	deleted := task.Task{ID: 7, Title: "gone", Status: "cancelled", Position: 4}
	result := runCommand(t, &fakeApplication{deleteResult: deleted}, "delete", "7", "--json")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	var decoded task.Task
	if err := json.Unmarshal([]byte(result.stdout), &decoded); err != nil {
		t.Fatalf("decode deleted task: %v", err)
	}
	if decoded != deleted {
		t.Errorf("deleted task = %#v, want %#v", decoded, deleted)
	}
}

func TestLifecycleValidationDoesNotOpenDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "invalid status", args: []string{"list", "--status", "later", "--json"}},
		{name: "done ID", args: []string{"done", "0", "--json"}},
		{name: "cancel ID", args: []string{"--json", "cancel", "--", "-1"}},
		{name: "reopen ID", args: []string{"reopen", "+1", "--json"}},
		{name: "delete ID", args: []string{"delete", "nope", "--json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, &fakeApplication{}, test.args...)
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
		})
	}
}

func TestHumanOutputUsesPlainTables(t *testing.T) {
	t.Parallel()

	dueOn := "2026-07-28"
	deferUntil := "2026-07-29"
	application := &fakeApplication{inboxResult: []task.ViewTask{
		{Task: task.Task{ID: 1, Title: "one", DueOn: &dueOn, DeferUntil: &deferUntil}},
		{Task: task.Task{ID: 20, Title: "twenty"}},
	}}
	result := runCommand(t, application, "inbox")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
	}
	if !strings.Contains(result.stdout, "1") || !strings.Contains(result.stdout, "twenty") {
		t.Errorf("stdout = %q, want task rows", result.stdout)
	}
	if !strings.Contains(result.stdout, "due 2026-07-28 defer 2026-07-29") {
		t.Errorf("stdout = %q, want compact due-then-defer tokens", result.stdout)
	}
	if strings.Contains(result.stdout, "\x1b[") {
		t.Errorf("stdout = %q, want no ANSI sequences", result.stdout)
	}
	for _, line := range strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("stdout line = %q, want no trailing spaces", line)
		}
	}
}

func TestHumanShowUsesPlainFieldValueTableForMultilineNotes(t *testing.T) {
	t.Parallel()

	dueOn := "2026-07-28"
	deferUntil := "2026-07-29"
	result := runCommand(t, &fakeApplication{showResult: task.Task{
		ID:         7,
		Title:      "capture",
		Note:       "first line\nsecond line\n",
		DueOn:      &dueOn,
		DeferUntil: &deferUntil,
		Status:     "open",
		Position:   2,
		CreatedAt:  "2026-07-27T12:00:00.000Z",
		UpdatedAt:  "2026-07-27T13:00:00.000Z",
	}}, "show", "7")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	for _, value := range []string{
		"ID",
		"Title",
		"Note",
		"first line",
		"second line",
		"Status",
		"Created at",
		"Updated at",
	} {
		if !strings.Contains(result.stdout, value) {
			t.Errorf("stdout = %q, want %q", result.stdout, value)
		}
	}
	normalizedRows := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n") {
		normalizedRows[strings.Join(strings.Fields(line), " ")] = true
	}
	for _, row := range []string{"Due on 2026-07-28", "Defer until 2026-07-29"} {
		if !normalizedRows[row] {
			t.Errorf("stdout = %q, want associated row %q", result.stdout, row)
		}
	}
	if strings.Contains(result.stdout, "\x1b[") {
		t.Errorf("stdout = %q, want no ANSI sequences", result.stdout)
	}
	for _, line := range strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("stdout line = %q, want no trailing spaces", line)
		}
	}
}

func TestHumanOutputEscapesTerminalControls(t *testing.T) {
	t.Parallel()

	title := "safe\x1b[31m\rforged\nrow"
	note := "first line\nsecond line\x1b]8;;https://example.com\a"
	tests := []struct {
		name        string
		application *fakeApplication
		args        []string
	}{
		{
			name:        "mutation",
			application: &fakeApplication{addResult: task.Task{ID: 1, Title: title}},
			args:        []string{"add", "input"},
		},
		{
			name: "collection",
			application: &fakeApplication{inboxResult: []task.ViewTask{
				{Task: task.Task{ID: 1, Title: title}},
			}},
			args: []string{"inbox"},
		},
		{
			name: "detail",
			application: &fakeApplication{showResult: task.Task{
				ID:     1,
				Title:  title,
				Note:   note,
				Status: "open",
			}},
			args: []string{"show", "1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, test.application, test.args...)
			if result.exitCode != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
			}
			if strings.ContainsAny(result.stdout, "\x1b\r\a") {
				t.Errorf("stdout = %q, want terminal controls escaped", result.stdout)
			}
			if !strings.Contains(result.stdout, `\x1b`) || !strings.Contains(result.stdout, `forged\nrow`) {
				t.Errorf("stdout = %q, want visible control escapes", result.stdout)
			}
		})
	}
}

func TestJSONErrorsUseStableCodesAndStreams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		application *fakeApplication
		args        []string
		wantCode    apperr.Code
		wantMessage string
		wantExit    int
	}{
		{
			name: "not found",
			application: &fakeApplication{showError: apperr.New(
				apperr.NotFound,
				"no task 99",
				nil,
			)},
			args:        []string{"show", "99", "--json"},
			wantCode:    apperr.NotFound,
			wantMessage: "no task 99",
			wantExit:    1,
		},
		{
			name: "conflict",
			application: &fakeApplication{doneError: apperr.New(
				apperr.Conflict,
				"task 1 is not open",
				nil,
			)},
			args:        []string{"done", "1", "--json"},
			wantCode:    apperr.Conflict,
			wantMessage: "task 1 is not open",
			wantExit:    1,
		},
		{
			name:        "internal",
			application: &fakeApplication{inboxError: errors.New("open database: permission denied")},
			args:        []string{"inbox", "--json"},
			wantCode:    apperr.Internal,
			wantMessage: "open database: permission denied",
			wantExit:    1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, test.application, test.args...)
			if result.exitCode != test.wantExit {
				t.Errorf("exit code = %d, want %d", result.exitCode, test.wantExit)
			}
			if result.stdout != "" {
				t.Errorf("stdout = %q, want empty", result.stdout)
			}
			var envelope errorEnvelope
			if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
				t.Fatalf("decode stderr: %v", err)
			}
			if envelope.Error.Code != test.wantCode {
				t.Errorf("error code = %q, want %q", envelope.Error.Code, test.wantCode)
			}
			if envelope.Error.Message != test.wantMessage {
				t.Errorf("error message = %q, want diagnostic %q", envelope.Error.Message, test.wantMessage)
			}
		})
	}
}

func TestHumanInternalErrorRetainsDiagnosticContext(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{inboxError: errors.New("open database: permission denied")}, "inbox")
	if result.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.exitCode)
	}
	if result.stdout != "" {
		t.Errorf("stdout = %q, want empty", result.stdout)
	}
	if result.stderr != "Error: open database: permission denied\n" {
		t.Errorf("stderr = %q, want actionable diagnostic", result.stderr)
	}
}

func TestInvalidIDIsApplicationErrorWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{}, "show", "0", "--json")
	if result.exitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.exitCode)
	}
	if result.opens != 0 {
		t.Errorf("factory opens = %d, want 0", result.opens)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode stderr: %v", err)
	}
	if envelope.Error.Code != apperr.InvalidArgument {
		t.Errorf("error code = %q, want invalid_argument", envelope.Error.Code)
	}
}

func TestUsageErrorsAreHumanReadableEvenWithJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "json unparsed", args: []string{"--unknown", "--json"}},
		{name: "json parsed", args: []string{"--json", "--unknown"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, &fakeApplication{}, test.args...)
			if result.exitCode != 2 {
				t.Errorf("exit code = %d, want 2", result.exitCode)
			}
			if result.stdout != "" {
				t.Errorf("stdout = %q, want empty", result.stdout)
			}
			if strings.HasPrefix(result.stderr, "{") {
				t.Errorf("stderr = %q, want human-readable diagnostic, not JSON", result.stderr)
			}
			if !strings.Contains(result.stderr, "unknown flag: --unknown") {
				t.Errorf("stderr = %q, want parse diagnostic", result.stderr)
			}
		})
	}
}

func TestHelpAndVersionDoNotOpenDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "bare root"},
		{name: "help", args: []string{"--db", "/unusable/path/gsd.db", "--help"}},
		{name: "version", args: []string{"--db", "/unusable/path/gsd.db", "--version"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCommand(t, &fakeApplication{}, test.args...)
			if result.exitCode != 0 {
				t.Errorf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
			}
			if result.opens != 0 {
				t.Errorf("factory opens = %d, want 0", result.opens)
			}
			if result.stderr != "" {
				t.Errorf("stderr = %q, want empty", result.stderr)
			}
		})
	}
}
