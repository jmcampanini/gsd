package cmd

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
)

type fakeAreaApplication struct {
	addResult   area.Area
	addError    error
	listResult  []area.Area
	listError   error
	showResult  area.Area
	showError   error
	editResult  area.Area
	editError   error
	addFields   area.AddFields
	listOptions area.ListOptions
	showID      int64
	editID      int64
	editFields  area.EditFields
}

func (f *fakeAreaApplication) Add(
	_ context.Context,
	fields area.AddFields,
) (area.Area, error) {
	f.addFields = fields
	return f.addResult, f.addError
}

func (f *fakeAreaApplication) List(
	_ context.Context,
	options area.ListOptions,
) ([]area.Area, error) {
	f.listOptions = options
	return f.listResult, f.listError
}

func (f *fakeAreaApplication) Show(_ context.Context, id int64) (area.Area, error) {
	f.showID = id
	return f.showResult, f.showError
}

func (f *fakeAreaApplication) Edit(
	_ context.Context,
	id int64,
	fields area.EditFields,
) (area.Area, error) {
	f.editID = id
	f.editFields = fields
	return f.editResult, f.editError
}

func runAreaCommand(t *testing.T, application area.Application, args ...string) commandResult {
	t.Helper()
	return runAreaCommandWithInput(t, application, strings.NewReader(""), args...)
}

func runAreaCommandWithInput(
	t *testing.T,
	application area.Application,
	input io.Reader,
	args ...string,
) commandResult {
	t.Helper()
	return runCommandWithApplications(t, applications{areas: application}, input, args...)
}

func decodeAreaJSON[T any](t *testing.T, output string) T {
	t.Helper()
	if !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("output = %q, want one newline-terminated JSON value", output)
	}

	var decoded T
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("decode area command output: %v", err)
	}
	return decoded
}

func decodeAreaCommandError(t *testing.T, result commandResult) errorPayload {
	t.Helper()
	if result.exitCode != 1 || result.stdout != "" {
		t.Fatalf("result = %#v, want stderr-only application error", result)
	}
	return decodeAreaJSON[errorEnvelope](t, result.stderr).Error
}

func TestAreaAddAdaptsStdinNoteAndWritesCompleteJSONRow(t *testing.T) {
	t.Parallel()

	note := "first line\nsecond line\n"
	created := area.Area{
		ID:        7,
		Title:     "Home",
		Note:      note,
		Position:  2,
		CreatedAt: "2026-07-27T12:00:00.000Z",
		UpdatedAt: "2026-07-28T12:00:00.000Z",
	}
	application := &fakeAreaApplication{addResult: created}
	result := runAreaCommandWithInput(
		t,
		application,
		strings.NewReader(note),
		"areas", "add", "Home", "--note", "-", "--db", "chosen.db", "--json",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.addFields != (area.AddFields{Title: "Home", Note: note}) {
		t.Errorf("Add() fields = %#v, want exact title and stdin note", application.addFields)
	}
	if got := decodeAreaJSON[area.Area](t, result.stdout); !reflect.DeepEqual(got, created) {
		t.Errorf("JSON area = %#v, want %#v", got, created)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &fields); err != nil {
		t.Fatalf("decode area fields: %v", err)
	}
	for _, field := range []string{"id", "title", "note", "archived_at", "position", "created_at", "updated_at"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON fields = %v, missing %q", fields, field)
		}
	}
	if len(fields) != 7 || string(fields["archived_at"]) != "null" {
		t.Errorf("JSON fields = %v, want complete area row with null archived_at", fields)
	}
	if result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want chosen path and one open/close", result)
	}

	human := runAreaCommand(t, &fakeAreaApplication{addResult: area.Area{
		ID: 7, Title: "Home\x1b[31m",
	}}, "areas", "add", "Home")
	if human.exitCode != 0 || human.stderr != "" || human.stdout != "Added area 7: Home\\x1b[31m\n" {
		t.Errorf("human result = %#v, want escaped add narration", human)
	}
}

func TestAreaListUsesActiveSliceAndHumanArchiveMarker(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-07-28T12:00:00.000Z"
	application := &fakeAreaApplication{listResult: []area.Area{
		{ID: 1, Title: "Home"},
		{ID: 12, Title: "Work", ArchivedAt: &archivedAt},
	}}
	result := runAreaCommand(t, application, "areas", "list")
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.listOptions != (area.ListOptions{Slice: area.ListSliceActive}) {
		t.Errorf("List() options = %#v, want active slice", application.listOptions)
	}
	lines := strings.Split(strings.TrimSuffix(result.stdout, "\n"), "\n")
	if len(lines) != 2 || strings.Join(strings.Fields(lines[0]), " ") != "1 Home" ||
		strings.Join(strings.Fields(lines[1]), " ") != "12 Work archived" {
		t.Errorf("stdout = %q, want headerless ID/title/archive rows", result.stdout)
	}

	empty := runAreaCommand(t, &fakeAreaApplication{}, "areas", "list")
	if empty.exitCode != 0 || empty.stdout != "" || empty.stderr != "" {
		t.Errorf("empty result = %#v, want no output", empty)
	}
}

func TestAreaShowUsesSchemaOrderFieldValueTable(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-07-28T12:00:00.000Z"
	application := &fakeAreaApplication{showResult: area.Area{
		ID:         7,
		Title:      "Home",
		Note:       "first line\nsecond\tline\x1b",
		ArchivedAt: &archivedAt,
		Position:   2,
		CreatedAt:  "2026-07-27T12:00:00.000Z",
		UpdatedAt:  archivedAt,
	}}
	result := runAreaCommand(t, application, "area", "show", "007")
	if result.exitCode != 0 || result.stderr != "" || application.showID != 7 {
		t.Fatalf("result/ID = %#v/%d, want successful show for 7", result, application.showID)
	}
	wantLabels := []string{"ID", "Title", "Note", "Archived at", "Position", "Created at", "Updated at"}
	lastIndex := -1
	for _, label := range wantLabels {
		index := strings.Index(result.stdout, label)
		if index <= lastIndex {
			t.Errorf("stdout = %q, want %q after preceding schema field", result.stdout, label)
		}
		lastIndex = index
	}
	if !strings.Contains(result.stdout, "first line\n") ||
		!strings.Contains(result.stdout, "second\\tline\\x1b") || strings.Contains(result.stdout, "\x1b") {
		t.Errorf("stdout = %q, want linefeeds preserved and other controls escaped", result.stdout)
	}
}

func TestAreaEditAdaptsOnlyChangedFieldsAndWritesHumanOutput(t *testing.T) {
	t.Parallel()

	title := "House"
	note := "upstairs\ndownstairs\n"
	application := &fakeAreaApplication{editResult: area.Area{ID: 7, Title: title}}
	result := runAreaCommandWithInput(
		t,
		application,
		strings.NewReader(note),
		"area", "edit", "7", "--title", title, "--note", "-",
	)
	if result.exitCode != 0 || result.stderr != "" || result.stdout != "Edited: area 7  House\n" {
		t.Fatalf("result = %#v, want edit narration", result)
	}
	if application.editID != 7 || application.editFields.Title == nil ||
		*application.editFields.Title != title || application.editFields.Note == nil ||
		*application.editFields.Note != note {
		t.Errorf("Edit() ID/fields = %d/%#v, want only changed fields", application.editID, application.editFields)
	}
}

func TestAreaValidationFailsBeforeFactoryOpen(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "edit without fields", args: []string{"area", "edit", "7", "--json"}},
		{name: "invalid show ID", args: []string{"area", "show", "0", "--json"}},
		{name: "invalid edit ID", args: []string{"area", "edit", "nope", "--title", "x", "--json"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result := runAreaCommand(t, &fakeAreaApplication{}, test.args...)
			got := decodeAreaCommandError(t, result)
			if result.opens != 0 {
				t.Errorf("opens = %d, want validation before factory open", result.opens)
			}
			if got.Code != apperr.InvalidArgument {
				t.Errorf("error = %#v, want invalid_argument", got)
			}
		})
	}
}

func TestBareAreaParentsAreUsageErrorsWithoutOpeningDatabase(t *testing.T) {
	t.Parallel()

	for _, noun := range []string{"areas", "area"} {
		result := runAreaCommand(t, &fakeAreaApplication{}, noun, "--json")
		if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("%s result = %#v, want stderr-only usage error without open", noun, result)
		}
		if strings.HasPrefix(result.stderr, "{") {
			t.Errorf("%s stderr = %q, want human-readable usage diagnostic", noun, result.stderr)
		}
	}
}

func TestAreaApplicationErrorUsesStderrAndClosesOnce(t *testing.T) {
	t.Parallel()

	application := &fakeAreaApplication{showError: apperr.New(apperr.NotFound, "area 99 not found", nil)}
	result := runAreaCommand(t, application, "area", "show", "99", "--json")
	got := decodeAreaCommandError(t, result)
	if got.Code != apperr.NotFound || got.Message != "area 99 not found" {
		t.Errorf("error = %#v, want area not_found diagnostic", got)
	}
	if result.stderr == "" || result.stdout != "" || result.opens != 1 || result.closes != 1 {
		t.Errorf("result = %#v, want stderr and one open/close", result)
	}
}

var _ area.Application = (*fakeAreaApplication)(nil)
