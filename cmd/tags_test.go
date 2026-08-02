package cmd

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tag"
)

type fakeTagApplication struct {
	addResult    tag.Tag
	addError     error
	listResult   []tag.ListedTag
	listError    error
	renameResult tag.Tag
	renameError  error
	deleteResult tag.Deletion
	deleteError  error
	addName      string
	renameOld    string
	renameNew    string
	deleteName   string
	listCalls    int
}

func (f *fakeTagApplication) Add(_ context.Context, name string) (tag.Tag, error) {
	f.addName = name
	return f.addResult, f.addError
}

func (f *fakeTagApplication) List(context.Context) ([]tag.ListedTag, error) {
	f.listCalls++
	return f.listResult, f.listError
}

func (f *fakeTagApplication) Rename(_ context.Context, oldName, newName string) (tag.Tag, error) {
	f.renameOld = oldName
	f.renameNew = newName
	return f.renameResult, f.renameError
}

func (f *fakeTagApplication) Delete(_ context.Context, name string) (tag.Deletion, error) {
	f.deleteName = name
	return f.deleteResult, f.deleteError
}

func runTagCommand(t *testing.T, application tag.Application, args ...string) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{tags: application}, strings.NewReader(""), args...)
}

func decodeTagJSON[T any](t *testing.T, output string) T {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("output = %q, want one newline-terminated JSON value", output)
	}
	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode tag command output: %v", err)
	}
	return decoded
}

func TestTagsAddAdaptsNameAndWritesCompleteOutput(t *testing.T) {
	t.Parallel()

	created := tag.Tag{
		ID:        7,
		Title:     "errands",
		CreatedAt: "2026-07-27T12:00:00.000Z",
		UpdatedAt: "2026-07-27T12:00:00.000Z",
	}
	application := &fakeTagApplication{addResult: created}
	result := runTagCommand(t, application, "tags", "add", "errands", "--db", "chosen.db", "--json")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.addName != "errands" {
		t.Errorf("Add() name = %q, want exact argument", application.addName)
	}
	if got := decodeTagJSON[tag.Tag](t, result.stdout); !reflect.DeepEqual(got, created) {
		t.Errorf("JSON tag = %#v, want %#v", got, created)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &fields); err != nil {
		t.Fatalf("decode tag fields: %v", err)
	}
	for _, field := range []string{"id", "title", "created_at", "updated_at"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON fields = %v, missing %q", fields, field)
		}
	}
	if len(fields) != 4 {
		t.Errorf("JSON fields = %v, want complete four-field row", fields)
	}
	if result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want chosen path and one open/close", result)
	}

	human := runTagCommand(
		t,
		&fakeTagApplication{addResult: tag.Tag{Title: "errands\x1b[31m"}},
		"tags", "add", "errands",
	)
	if human.exitCode != 0 || human.stderr != "" || human.stdout != "Added tag errands\\x1b[31m\n" {
		t.Errorf("human result = %#v, want escaped add line", human)
	}

	blankApplication := &fakeTagApplication{addResult: tag.Tag{}}
	blank := runTagCommand(t, blankApplication, "tags", "add", "", "--json")
	if blank.exitCode != 0 || blank.stderr != "" || blankApplication.addName != "" || blank.opens != 1 {
		t.Errorf("blank result/call = %#v/%q, want name passed to service after open", blank, blankApplication.addName)
	}
}

func TestTagsListWritesCompleteJSONAndHeaderlessEscapedHumanRows(t *testing.T) {
	t.Parallel()

	listed := []tag.ListedTag{
		{Tag: tag.Tag{ID: 1, Title: "home", CreatedAt: "a", UpdatedAt: "b"}, UsageCount: 0},
		{Tag: tag.Tag{ID: 2, Title: "work\turgent", CreatedAt: "c", UpdatedAt: "d"}, UsageCount: 12},
	}
	application := &fakeTagApplication{listResult: listed}
	result := runTagCommand(t, application, "tags", "list", "--json")
	if result.exitCode != 0 || result.stderr != "" || application.listCalls != 1 {
		t.Fatalf("result/calls = %#v/%d, want one successful list", result, application.listCalls)
	}
	if got := decodeTagJSON[[]tag.ListedTag](t, result.stdout); !reflect.DeepEqual(got, listed) {
		t.Errorf("JSON tags = %#v, want %#v", got, listed)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &rows); err != nil {
		t.Fatalf("decode listed fields: %v", err)
	}
	for index, row := range rows {
		if len(row) != 5 || row["usage_count"] == nil {
			t.Errorf("row %d fields = %v, want tag row plus usage_count", index, row)
		}
	}

	human := runTagCommand(t, &fakeTagApplication{listResult: listed}, "tags", "list")
	if human.exitCode != 0 || human.stderr != "" || strings.Contains(human.stdout, "TITLE") {
		t.Fatalf("human result = %#v, want headerless success", human)
	}
	lines := strings.Split(strings.TrimSuffix(human.stdout, "\n"), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[0]), " ") != "home 0" ||
		strings.Join(strings.Fields(lines[1]), " ") != "work\\turgent 12" {
		t.Errorf("stdout = %q, want escaped name/count rows", human.stdout)
	}
	if strings.Contains(human.stdout, "\t") {
		t.Errorf("stdout = %q, want control characters escaped", human.stdout)
	}

	emptyHuman := runTagCommand(t, &fakeTagApplication{}, "tags", "list")
	if emptyHuman.exitCode != 0 || emptyHuman.stdout != "" || emptyHuman.stderr != "" {
		t.Errorf("empty human result = %#v, want no output", emptyHuman)
	}
	emptyJSON := runTagCommand(t, &fakeTagApplication{listResult: []tag.ListedTag{}}, "tags", "list", "--json")
	if emptyJSON.exitCode != 0 || emptyJSON.stdout != "[]\n" || emptyJSON.stderr != "" {
		t.Errorf("empty JSON result = %#v, want [] newline", emptyJSON)
	}
}

func TestTagsRenameAndDeleteAdaptArgumentsAndOutputShapes(t *testing.T) {
	t.Parallel()

	renamed := tag.Tag{ID: 3, Title: "out-and-about", CreatedAt: "a", UpdatedAt: "b"}
	renameApplication := &fakeTagApplication{renameResult: renamed}
	rename := runTagCommand(t, renameApplication, "tags", "rename", "errands", "out-and-about", "--json")
	if rename.exitCode != 0 || rename.stderr != "" || renameApplication.renameOld != "errands" ||
		renameApplication.renameNew != "out-and-about" {
		t.Fatalf("rename result/call = %#v/(%q,%q), want successful exact adaptation", rename, renameApplication.renameOld, renameApplication.renameNew)
	}
	if got := decodeTagJSON[tag.Tag](t, rename.stdout); !reflect.DeepEqual(got, renamed) {
		t.Errorf("rename JSON = %#v, want %#v", got, renamed)
	}
	renameHuman := runTagCommand(
		t,
		&fakeTagApplication{renameResult: renamed},
		"tags", "rename", "old\rname", "new\x1bname",
	)
	if renameHuman.exitCode != 0 || renameHuman.stderr != "" || renameHuman.stdout != "Renamed tag old\\rname to new\\x1bname\n" {
		t.Errorf("rename human = %#v, want escaped concise line", renameHuman)
	}

	deletion := tag.Deletion{Tag: renamed, Detached: 3}
	deleteApplication := &fakeTagApplication{deleteResult: deletion}
	deleted := runTagCommand(t, deleteApplication, "tags", "delete", "OUT-AND-ABOUT", "--json")
	if deleted.exitCode != 0 || deleted.stderr != "" || deleteApplication.deleteName != "OUT-AND-ABOUT" {
		t.Fatalf("delete result/call = %#v/%q, want successful exact adaptation", deleted, deleteApplication.deleteName)
	}
	if got := decodeTagJSON[tag.Deletion](t, deleted.stdout); !reflect.DeepEqual(got, deletion) {
		t.Errorf("delete JSON = %#v, want %#v", got, deletion)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(deleted.stdout), &envelope); err != nil {
		t.Fatalf("decode deletion fields: %v", err)
	}
	if len(envelope) != 2 || envelope["tag"] == nil || string(envelope["detached"]) != "3" {
		t.Errorf("deletion fields = %v, want tag/detached envelope", envelope)
	}

	for _, test := range []struct {
		count int64
		want  string
	}{
		{count: 0, want: "Deleted tag out-and-about (detached from 0 items)\n"},
		{count: 1, want: "Deleted tag out-and-about (detached from 1 item)\n"},
	} {
		human := runTagCommand(t, &fakeTagApplication{deleteResult: tag.Deletion{Tag: renamed, Detached: test.count}}, "tags", "delete", "out-and-about")
		if human.exitCode != 0 || human.stderr != "" || human.stdout != test.want {
			t.Errorf("delete %d result = %#v, want %q", test.count, human, test.want)
		}
	}
}

func TestTagsApplicationErrorsUseStderrAndCloseOnce(t *testing.T) {
	t.Parallel()

	applicationError := apperr.New(apperr.Conflict, "tag operation failed", nil)
	for _, test := range []struct {
		name        string
		application *fakeTagApplication
		args        []string
	}{
		{name: "add", application: &fakeTagApplication{addError: applicationError}, args: []string{"tags", "add", "Errands", "--json"}},
		{name: "list", application: &fakeTagApplication{listError: applicationError}, args: []string{"tags", "list", "--json"}},
		{name: "rename", application: &fakeTagApplication{renameError: applicationError}, args: []string{"tags", "rename", "old", "new", "--json"}},
		{name: "delete", application: &fakeTagApplication{deleteError: applicationError}, args: []string{"tags", "delete", "errands", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := runTagCommand(t, test.application, test.args...)
			if result.exitCode != 1 || result.stdout != "" || result.opens != 1 || result.closes != 1 {
				t.Fatalf("result = %#v, want stderr-only application error and one open/close", result)
			}
			envelope := decodeTagJSON[errorEnvelope](t, result.stderr)
			if envelope.Error.Code != apperr.Conflict || envelope.Error.Message != "tag operation failed" {
				t.Errorf("error = %#v, want conflict unchanged", envelope.Error)
			}
		})
	}
}

func TestTagsArityAndBareParentAreUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "bare parent", args: []string{"tags", "--json"}},
		{name: "add missing", args: []string{"tags", "add", "--json"}},
		{name: "add extra", args: []string{"tags", "add", "one", "two", "--json"}},
		{name: "list extra", args: []string{"tags", "list", "extra", "--json"}},
		{name: "rename missing", args: []string{"tags", "rename", "old", "--json"}},
		{name: "rename extra", args: []string{"tags", "rename", "old", "new", "extra", "--json"}},
		{name: "delete missing", args: []string{"tags", "delete", "--json"}},
		{name: "delete extra", args: []string{"tags", "delete", "one", "two", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := runTagCommand(t, &fakeTagApplication{}, test.args...)
			if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
				t.Errorf("result = %#v, want stderr-only usage error without open", result)
			}
			if strings.HasPrefix(result.stderr, "{") {
				t.Errorf("stderr = %q, want human-readable Cobra diagnostic", result.stderr)
			}
		})
	}
}

var _ tag.Application = (*fakeTagApplication)(nil)
