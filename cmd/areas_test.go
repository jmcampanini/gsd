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
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeAreaApplication struct {
	addResult       area.Area
	addError        error
	listResult      []area.Area
	listError       error
	showResult      area.Area
	showError       error
	editResult      area.Area
	editError       error
	archiveResult   area.Area
	archiveError    error
	unarchiveResult area.Area
	unarchiveError  error
	deleteResult    area.Deletion
	deleteError     error
	addFields       area.AddFields
	listOptions     area.ListOptions
	showID          int64
	editID          int64
	editFields      area.EditFields
	archiveID       int64
	unarchiveID     int64
	deleteID        int64
	deleteRecursive bool
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

func (f *fakeAreaApplication) Archive(_ context.Context, id int64) (area.Area, error) {
	f.archiveID = id
	return f.archiveResult, f.archiveError
}

func (f *fakeAreaApplication) Unarchive(_ context.Context, id int64) (area.Area, error) {
	f.unarchiveID = id
	return f.unarchiveResult, f.unarchiveError
}

func (f *fakeAreaApplication) Delete(
	_ context.Context,
	id int64,
	recursive bool,
) (area.Deletion, error) {
	f.deleteID = id
	f.deleteRecursive = recursive
	return f.deleteResult, f.deleteError
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

func TestAreaListFlagsMapDirectlyToSlicesAndConflictBeforeOpen(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		flag  string
		slice area.ListSlice
	}{
		{name: "archived", flag: "--archived", slice: area.ListSliceArchived},
		{name: "all", flag: "--all", slice: area.ListSliceAll},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			application := &fakeAreaApplication{listResult: []area.Area{}}
			result := runAreaCommand(t, application, "areas", "list", test.flag, "--json")
			if result.exitCode != 0 || result.stdout != "[]\n" || result.stderr != "" {
				t.Fatalf("result = %#v, want empty JSON success", result)
			}
			if application.listOptions.Slice != test.slice {
				t.Errorf("List() slice = %q, want %q", application.listOptions.Slice, test.slice)
			}
		})
	}

	conflict := runAreaCommand(
		t,
		&fakeAreaApplication{},
		"areas", "list", "--archived", "--all", "--json",
	)
	if conflict.exitCode != 2 || conflict.opens != 0 || conflict.stdout != "" || conflict.stderr == "" {
		t.Errorf("conflict = %#v, want stderr-only usage error without open", conflict)
	}
	if strings.HasPrefix(conflict.stderr, "{") {
		t.Errorf("stderr = %q, want human-readable Cobra diagnostic", conflict.stderr)
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

func TestAreaArchiveAndUnarchiveAdaptLifecycleAndOutput(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-07-28T12:00:00.000Z"
	archived := area.Area{ID: 7, Title: "Home\x1b", ArchivedAt: &archivedAt, Position: 2}
	archiveApplication := &fakeAreaApplication{archiveResult: archived}
	archive := runAreaCommand(t, archiveApplication, "area", "archive", "007")
	if archive.exitCode != 0 || archive.stderr != "" || archive.stdout != "Archived: area 7  Home\\x1b\n" {
		t.Fatalf("archive = %#v, want escaped mutation narration", archive)
	}
	if archiveApplication.archiveID != 7 || archive.opens != 1 || archive.closes != 1 {
		t.Errorf("archive call/lifecycle = %d/%#v, want ID 7 and one open/close", archiveApplication.archiveID, archive)
	}

	unarchived := area.Area{ID: 7, Title: "Home", Position: 2}
	unarchiveApplication := &fakeAreaApplication{unarchiveResult: unarchived}
	unarchive := runAreaCommand(t, unarchiveApplication, "area", "unarchive", "7", "--json")
	if unarchive.exitCode != 0 || unarchive.stderr != "" || unarchiveApplication.unarchiveID != 7 {
		t.Fatalf("unarchive = %#v, want JSON success for ID 7", unarchive)
	}
	if got := decodeAreaJSON[area.Area](t, unarchive.stdout); !reflect.DeepEqual(got, unarchived) {
		t.Errorf("unarchive JSON = %#v, want full row %#v", got, unarchived)
	}
}

func TestAreaDeleteSelectsJSONShapeAndAdaptsRecursive(t *testing.T) {
	t.Parallel()

	deletion := area.Deletion{
		Area:            area.Area{ID: 7, Title: "Home", Position: 2},
		DeletedProjects: []project.Project{{ID: 3, Title: "Kitchen", Position: 1}},
		DeletedTasks:    []task.Task{{ID: 5, Title: "Quotes", Position: 0}},
	}

	nonrecursiveApplication := &fakeAreaApplication{deleteResult: deletion}
	nonrecursive := runAreaCommand(t, nonrecursiveApplication, "area", "delete", "7", "--json")
	if nonrecursive.exitCode != 0 || nonrecursive.stderr != "" {
		t.Fatalf("nonrecursive = %#v, want JSON success", nonrecursive)
	}
	if got := decodeAreaJSON[area.Area](t, nonrecursive.stdout); !reflect.DeepEqual(got, deletion.Area) {
		t.Errorf("nonrecursive JSON = %#v, want area row %#v", got, deletion.Area)
	}
	if nonrecursiveApplication.deleteID != 7 || nonrecursiveApplication.deleteRecursive {
		t.Errorf("Delete() input = (%d, %t), want (7, false)", nonrecursiveApplication.deleteID, nonrecursiveApplication.deleteRecursive)
	}

	recursiveApplication := &fakeAreaApplication{deleteResult: deletion}
	recursive := runAreaCommand(t, recursiveApplication, "area", "delete", "7", "--recursive", "--json")
	if recursive.exitCode != 0 || recursive.stderr != "" {
		t.Fatalf("recursive = %#v, want JSON success", recursive)
	}
	if got := decodeAreaJSON[area.Deletion](t, recursive.stdout); !reflect.DeepEqual(got, deletion) {
		t.Errorf("recursive JSON = %#v, want envelope %#v", got, deletion)
	}
	if recursiveApplication.deleteID != 7 || !recursiveApplication.deleteRecursive {
		t.Errorf("Delete() input = (%d, %t), want (7, true)", recursiveApplication.deleteID, recursiveApplication.deleteRecursive)
	}
}

func TestAreaDeleteHumanNarrationIncludesOnlyNonemptySections(t *testing.T) {
	t.Parallel()

	deletion := area.Deletion{
		Area: area.Area{ID: 7, Title: "Home"},
		DeletedProjects: []project.Project{
			{ID: 2, Title: "Kitchen\x1b"},
			{ID: 3, Title: "Garden"},
		},
		DeletedTasks: []task.Task{{ID: 4, Title: "Quotes\rforged"}},
	}
	result := runAreaCommand(
		t,
		&fakeAreaApplication{deleteResult: deletion},
		"area", "delete", "7", "--recursive",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	for _, text := range []string{
		"Deleted: area 7  Home\n",
		"Deleted 2 projects:\n",
		"Deleted 1 task:\n",
		"Kitchen\\x1b",
		"Quotes\\rforged",
	} {
		if !strings.Contains(result.stdout, text) {
			t.Errorf("stdout = %q, want %q", result.stdout, text)
		}
	}
	if strings.ContainsAny(result.stdout, "\x1b\r") {
		t.Errorf("stdout = %q, want terminal controls escaped", result.stdout)
	}

	empty := runAreaCommand(
		t,
		&fakeAreaApplication{deleteResult: area.Deletion{Area: area.Area{ID: 8, Title: "Empty"}}},
		"area", "delete", "8", "--recursive",
	)
	if empty.exitCode != 0 || empty.stderr != "" || empty.stdout != "Deleted: area 8  Empty\n" {
		t.Errorf("empty recursive deletion = %#v, want mutation line without empty sections", empty)
	}
}

func TestAreaDeleteConflictAddsRecursiveRecovery(t *testing.T) {
	t.Parallel()

	application := &fakeAreaApplication{deleteError: apperr.New(
		apperr.Conflict,
		"cannot delete area 7 while it contains projects or tasks",
		nil,
	)}
	result := runAreaCommand(t, application, "area", "delete", "7", "--json")
	got := decodeAreaCommandError(t, result)
	if got.Code != apperr.Conflict || got.Message != "cannot delete area 7 while it contains projects or tasks; use --recursive to delete the area and its contents" {
		t.Errorf("error = %#v, want recursive recovery guidance", got)
	}
	if application.deleteID != 7 || application.deleteRecursive || result.opens != 1 || result.closes != 1 {
		t.Errorf("call/lifecycle = (%d, %t)/%#v, want nonrecursive call and one open/close", application.deleteID, application.deleteRecursive, result)
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
		{name: "invalid archive ID", args: []string{"area", "archive", "0", "--json"}},
		{name: "invalid unarchive ID", args: []string{"area", "unarchive", "nope", "--json"}},
		{name: "invalid delete ID", args: []string{"area", "delete", "+1", "--json"}},
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
