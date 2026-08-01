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

	"github.com/jmcampanini/gsd/internal/task"
)

type fakeApplication struct {
	addResult    task.Task
	addError     error
	inboxResult  []task.Task
	inboxError   error
	listResult   []task.Task
	listError    error
	showResult   task.Task
	showError    error
	editResult   task.Task
	editError    error
	doneResult   task.Task
	doneError    error
	cancelResult task.Task
	cancelError  error
	reopenResult task.Task
	reopenError  error
	deleteResult task.Task
	deleteError  error
	addTitle     string
	addNote      string
	addDueOn     *string
	listOptions  task.ListOptions
	showID       int64
	editID       int64
	editFields   task.EditFields
	mutation     string
	mutationID   int64
}

func (f *fakeApplication) Add(_ context.Context, fields task.AddFields) (task.Task, error) {
	f.addTitle = fields.Title
	f.addNote = fields.Note
	f.addDueOn = fields.DueOn
	return f.addResult, f.addError
}

func (f *fakeApplication) Inbox(context.Context) ([]task.Task, error) {
	return f.inboxResult, f.inboxError
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

func runCommandWithInput(
	t *testing.T,
	application task.Application,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result := commandResult{}
	factory := func(_ context.Context, path string) (task.Application, io.Closer, error) {
		result.opens++
		result.openPath = path
		return application, closeRecorder{close: func() { result.closes++ }}, nil
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
			err:  task.NewError(task.ErrorConflict, "conflict", nil),
			want: 1,
		},
		{
			name: "wrapped application",
			err:  errors.Join(errors.New("context"), task.NewError(task.ErrorNotFound, "missing", nil)),
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

func TestJSONCommandOutput(t *testing.T) {
	t.Parallel()

	dueOn := "2026-07-28"
	created := task.Task{
		ID:        7,
		Title:     "capture",
		Note:      "details",
		DueOn:     &dueOn,
		Status:    "open",
		Position:  2,
		CreatedAt: "2026-07-27T12:00:00.000Z",
		UpdatedAt: "2026-07-27T12:00:00.000Z",
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
	if len(fields) != 11 {
		t.Errorf("JSON field count = %d, want 11", len(fields))
	}
	if string(fields["defer_until"]) != "null" ||
		string(fields["done_at"]) != "null" ||
		string(fields["cancelled_at"]) != "null" {
		t.Errorf("nullable fields = (%s, %s, %s), want null", fields["defer_until"], fields["done_at"], fields["cancelled_at"])
	}
	if application.addTitle != "capture" || application.addNote != "details" ||
		application.addDueOn == nil || *application.addDueOn != "tomorrow" {
		t.Errorf("Add() input = (%q, %q, %#v), want exact command input", application.addTitle, application.addNote, application.addDueOn)
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
	if envelope.Error.Code != task.ErrorInternal {
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

func TestEditWithoutFieldsReturnsApplicationError(t *testing.T) {
	t.Parallel()

	application := &fakeApplication{editError: task.NewError(
		task.ErrorInvalidArgument,
		"edit requires at least one field",
		nil,
	)}
	result := runCommand(t, application, "edit", "7", "--json")
	if result.exitCode != 1 || result.stdout != "" || result.opens != 1 || result.closes != 1 {
		t.Errorf("result = %#v, want stderr-only invalid_argument with one application lifecycle", result)
	}
	if application.editFields.Title != nil || application.editFields.Note != nil {
		t.Errorf("Edit() fields = %#v, want both omitted", application.editFields)
	}
	var envelope errorEnvelope
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if envelope.Error.Code != task.ErrorInvalidArgument {
		t.Errorf("error code = %q, want invalid_argument", envelope.Error.Code)
	}
}

func TestEmptyInboxJSONIsArray(t *testing.T) {
	t.Parallel()

	result := runCommand(t, &fakeApplication{inboxResult: []task.Task{}}, "inbox", "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", result.exitCode, result.stderr)
	}
	if result.stdout != "[]\n" {
		t.Errorf("stdout = %q, want empty JSON array", result.stdout)
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

func TestDeadlineFlagConflictsAreUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "edit", args: []string{"edit", "7", "--due", "today", "--no-due", "--json"}},
		{name: "list", args: []string{"list", "--due", "--overdue", "--json"}},
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
			if envelope.Error.Code != task.ErrorInvalidArgument {
				t.Errorf("error code = %q, want invalid_argument", envelope.Error.Code)
			}
		})
	}
}

func TestHumanOutputUsesPlainTables(t *testing.T) {
	t.Parallel()

	dueOn := "2026-07-28"
	deferUntil := "2026-07-29"
	application := &fakeApplication{inboxResult: []task.Task{
		{ID: 1, Title: "one", DueOn: &dueOn, DeferUntil: &deferUntil},
		{ID: 20, Title: "twenty"},
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
			application: &fakeApplication{inboxResult: []task.Task{
				{ID: 1, Title: title},
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
		wantCode    task.ErrorCode
		wantMessage string
		wantExit    int
	}{
		{
			name: "not found",
			application: &fakeApplication{showError: task.NewError(
				task.ErrorNotFound,
				"no task 99",
				nil,
			)},
			args:        []string{"show", "99", "--json"},
			wantCode:    task.ErrorNotFound,
			wantMessage: "no task 99",
			wantExit:    1,
		},
		{
			name: "conflict",
			application: &fakeApplication{doneError: task.NewError(
				task.ErrorConflict,
				"task 1 is not open",
				nil,
			)},
			args:        []string{"done", "1", "--json"},
			wantCode:    task.ErrorConflict,
			wantMessage: "task 1 is not open",
			wantExit:    1,
		},
		{
			name:        "internal",
			application: &fakeApplication{inboxError: errors.New("open database: permission denied")},
			args:        []string{"inbox", "--json"},
			wantCode:    task.ErrorInternal,
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
	if envelope.Error.Code != task.ErrorInvalidArgument {
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
