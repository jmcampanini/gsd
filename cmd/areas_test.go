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
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

type fakeAreaApplication struct {
	addResult        area.Area
	addError         error
	listResult       []area.Area
	listError        error
	showResult       area.Area
	showError        error
	editResult       area.Area
	editError        error
	archiveResult    area.Area
	archiveError     error
	unarchiveResult  area.Area
	unarchiveError   error
	reorderResult    area.Area
	reorderError     error
	deleteResult     area.Deletion
	deleteError      error
	tagResult        area.Tagging
	tagError         error
	untagResult      area.Tagging
	untagError       error
	addFields        area.AddFields
	listOptions      area.ListOptions
	showID           int64
	editID           int64
	editFields       area.EditFields
	archiveID        int64
	unarchiveID      int64
	reorderID        int64
	reorderPlacement domain.Placement
	deleteID         int64
	deleteRecursive  bool
	tagID            int64
	tagNames         []string
	untagID          int64
	untagNames       []string
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

func (f *fakeAreaApplication) Reorder(
	_ context.Context,
	id int64,
	placement domain.Placement,
) (area.Area, error) {
	f.reorderID = id
	f.reorderPlacement = placement
	return f.reorderResult, f.reorderError
}

func (f *fakeAreaApplication) Tag(
	_ context.Context,
	id int64,
	names []string,
) (area.Tagging, error) {
	f.tagID = id
	f.tagNames = append([]string(nil), names...)
	return f.tagResult, f.tagError
}

func (f *fakeAreaApplication) Untag(
	_ context.Context,
	id int64,
	names []string,
) (area.Tagging, error) {
	f.untagID = id
	f.untagNames = append([]string(nil), names...)
	return f.untagResult, f.untagError
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
		Tags:      []string{},
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
	if !reflect.DeepEqual(application.addFields, area.AddFields{Title: "Home", Note: note}) {
		t.Errorf("Add() fields = %#v, want exact title and stdin note", application.addFields)
	}
	if got := decodeAreaJSON[area.Area](t, result.stdout); !reflect.DeepEqual(got, created) {
		t.Errorf("JSON area = %#v, want %#v", got, created)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &fields); err != nil {
		t.Fatalf("decode area fields: %v", err)
	}
	for _, field := range []string{"id", "title", "note", "archived_at", "position", "created_at", "updated_at", "tags"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("JSON fields = %v, missing %q", fields, field)
		}
	}
	if len(fields) != 8 || string(fields["archived_at"]) != "null" || string(fields["tags"]) != "[]" {
		t.Errorf("JSON fields = %v, want complete area row with null archived_at and empty tags", fields)
	}
	if result.openPath != "chosen.db" || result.opens != 1 || result.closes != 1 {
		t.Errorf("factory lifecycle = %#v, want chosen path and one open/close", result)
	}

	human := runAreaCommand(t, &fakeAreaApplication{addResult: area.Area{
		ID: 7, Title: "Home\x1b[31m",
	}}, "areas", "add", "Home")
	if human.exitCode != 0 || human.stderr != "" || human.stdout != "+ Added area 7: Home\\x1b[31m\n" {
		t.Errorf("human result = %#v, want escaped add narration", human)
	}
}

func TestAreaAddAccumulatesTagFlagsWithoutSplittingCommas(t *testing.T) {
	t.Parallel()

	application := &fakeAreaApplication{addResult: area.Area{
		ID:    8,
		Title: "Tagged",
		Tags:  []string{"Errands", "Home,House"},
	}}
	result := runAreaCommand(
		t,
		application,
		"areas", "add", "Tagged", "--tag", "Errands", "--tag", "home,house", "--tag", "WORK",
	)
	if result.exitCode != 0 || result.stderr != "" ||
		!strings.Contains(humanFields(result.stdout), "Added area 8: Tagged #Errands #Home,House") {
		t.Fatalf("result = %#v, want tagged add narration", result)
	}
	want := area.AddFields{Title: "Tagged", Tags: []string{"Errands", "home,house", "WORK"}}
	if !reflect.DeepEqual(application.addFields, want) {
		t.Errorf("Add() fields = %#v, want %#v", application.addFields, want)
	}
}

func TestAreaTagAdaptsExactNamesAndWritesOnlyAreaJSON(t *testing.T) {
	t.Parallel()

	affected := area.Area{
		ID:    7,
		Title: "Home",
		Tags:  []string{"Errands", "Home,House"},
	}
	application := &fakeAreaApplication{tagResult: area.Tagging{
		Area:      affected,
		TagTitles: []string{"Errands", "Home,House"},
	}}
	result := runAreaCommand(
		t,
		application,
		"area", "tag", "007", "errands", "Home,House", "--json",
	)
	if result.exitCode != 0 || result.stderr != "" {
		t.Fatalf("result = %#v, want success", result)
	}
	if application.tagID != 7 || !reflect.DeepEqual(application.tagNames, []string{"errands", "Home,House"}) {
		t.Errorf("Tag() input = %d/%#v, want exact ID and names", application.tagID, application.tagNames)
	}
	if got := decodeAreaJSON[area.Area](t, result.stdout); !reflect.DeepEqual(got, affected) {
		t.Errorf("tag JSON = %#v, want area only %#v", got, affected)
	}
}

func TestAreaUntagUsesStoredSpellingsForHumanOutput(t *testing.T) {
	t.Parallel()

	application := &fakeAreaApplication{untagResult: area.Tagging{
		Area:      area.Area{ID: 7, Title: "Home"},
		TagTitles: []string{"Stored\x1b", "Second\rName"},
	}}
	result := runAreaCommand(t, application, "area", "untag", "7", "stored", "second")
	if result.exitCode != 0 || result.stderr != "" ||
		!strings.Contains(humanFields(result.stdout), "Untagged: area 7 #Stored\\x1b #Second\\rName") {
		t.Fatalf("result = %#v, want escaped stored tag spellings", result)
	}
	if application.untagID != 7 || !reflect.DeepEqual(application.untagNames, []string{"stored", "second"}) {
		t.Errorf("Untag() input = %d/%#v, want exact ID and names", application.untagID, application.untagNames)
	}
}

func TestAreaTagArityFailsBeforeFactoryOpen(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"area", "tag"},
		{"area", "tag", "7"},
		{"area", "untag", "7"},
	} {
		result := runAreaCommand(t, &fakeAreaApplication{}, args...)
		if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("%v result = %#v, want stderr-only usage error without open", args, result)
		}
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
	if len(lines) != 3 || strings.Join(strings.Fields(lines[0]), " ") != "id title state" ||
		strings.Join(strings.Fields(lines[1]), " ") != "1 Home" ||
		strings.Join(strings.Fields(lines[2]), " ") != "12 Work archived" {
		t.Errorf("stdout = %q, want headed ID/title/state rows", result.stdout)
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

func TestAreaShowUsesStatusGlyphOutline(t *testing.T) {
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
		Tags:       []string{"Errands", "Home"},
	}}
	result := runAreaCommand(t, application, "area", "show", "007")
	if result.exitCode != 0 || result.stderr != "" || application.showID != 7 {
		t.Fatalf("result/ID = %#v/%d, want successful show for 7", result, application.showID)
	}
	normalized := humanFields(result.stdout)
	for _, fragment := range []string{
		"7 Home",
		"note first line second\\tline\\x1b",
		"archived at 2026-07-28T12:00:00.000Z",
		"position 2",
		"created at 2026-07-27T12:00:00.000Z",
		"updated at 2026-07-28T12:00:00.000Z",
		"tags #Errands #Home",
	} {
		if !strings.Contains(normalized, fragment) {
			t.Errorf("stdout = %q, want %q", result.stdout, fragment)
		}
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
	if result.exitCode != 0 || result.stderr != "" ||
		!strings.Contains(humanFields(result.stdout), "Edited: area 7 House") {
		t.Fatalf("result = %#v, want edit narration", result)
	}
	if application.editID != 7 || application.editFields.Title == nil ||
		*application.editFields.Title != title || application.editFields.Note == nil ||
		*application.editFields.Note != note {
		t.Errorf("Edit() ID/fields = %d/%#v, want only changed fields", application.editID, application.editFields)
	}
}

func TestAreaReorderAdaptsEveryPlacementAndWritesBareJSON(t *testing.T) {
	t.Parallel()

	wantArea := area.Area{
		ID: 7, Title: "Home", Position: 3, Tags: domain.TagNames{},
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

			application := &fakeAreaApplication{reorderResult: wantArea}
			result := runAreaCommand(t, application, "area", "reorder", "007", test.flag, "--json")
			if result.exitCode != 0 || result.stderr != "" || result.opens != 1 || result.closes != 1 {
				t.Fatalf("result = %#v, want JSON success and one factory lifecycle", result)
			}
			if application.reorderID != 7 || !reflect.DeepEqual(application.reorderPlacement, test.want) {
				t.Errorf("Reorder() input = (%d, %#v), want (7, %#v)", application.reorderID, application.reorderPlacement, test.want)
			}
			if got := decodeAreaJSON[area.Area](t, result.stdout); !reflect.DeepEqual(got, wantArea) {
				t.Errorf("reorder JSON = %#v, want bare area %#v", got, wantArea)
			}
		})
	}
}

func TestAreaReorderWritesExactHumanMutation(t *testing.T) {
	t.Parallel()

	result := runAreaCommand(
		t,
		&fakeAreaApplication{reorderResult: area.Area{ID: 7, Title: "Home"}},
		"area", "reorder", "7", "--first",
	)
	if result.exitCode != 0 || result.stdout != "~ Reordered: area 7  Home\n" || result.stderr != "" {
		t.Errorf("result = %#v, want exact stdout-only reorder mutation", result)
	}
}

func TestAreaReorderApplicationErrorUsesOnlyStderr(t *testing.T) {
	t.Parallel()

	application := &fakeAreaApplication{reorderError: apperr.New(apperr.NotFound, "no area 99", nil)}
	result := runAreaCommand(t, application, "area", "reorder", "99", "--last", "--json")
	got := decodeAreaCommandError(t, result)
	if got.Code != apperr.NotFound || got.Message != "no area 99" {
		t.Errorf("error = %#v, want area not_found diagnostic", got)
	}
	if result.exitCode != 1 || result.stdout != "" || result.opens != 1 || result.closes != 1 {
		t.Errorf("result = %#v, want stderr-only application failure and one factory lifecycle", result)
	}
}

func TestAreaArchiveAndUnarchiveAdaptLifecycleAndOutput(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-07-28T12:00:00.000Z"
	archived := area.Area{ID: 7, Title: "Home\x1b", ArchivedAt: &archivedAt, Position: 2}
	archiveApplication := &fakeAreaApplication{archiveResult: archived}
	archive := runAreaCommand(t, archiveApplication, "area", "archive", "007")
	if archive.exitCode != 0 || archive.stderr != "" ||
		!strings.Contains(humanFields(archive.stdout), "Archived: area 7 Home\\x1b") {
		t.Fatalf("archive = %#v, want escaped mutation narration", archive)
	}
	if archiveApplication.archiveID != 7 || archive.opens != 1 || archive.closes != 1 {
		t.Errorf("archive call/lifecycle = %d/%#v, want ID 7 and one open/close", archiveApplication.archiveID, archive)
	}

	unarchived := area.Area{ID: 7, Title: "Home", Position: 2, Tags: domain.TagNames{}}
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
		Area: area.Area{ID: 7, Title: "Home", Position: 2, Tags: domain.TagNames{}},
		DeletedProjects: []project.Project{{
			ID: 3, Title: "Kitchen", Position: 1, Tags: domain.TagNames{},
		}},
		DeletedTasks: []task.Task{{
			ID: 5, Title: "Quotes", Position: 0, Tags: domain.TagNames{},
		}},
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
	deletionNormalized := humanFields(result.stdout)
	for _, text := range []string{
		"Deleted: area 7 Home",
		"Deleted 2 projects:",
		"Deleted 1 task:",
		"Kitchen\\x1b",
		"Quotes\\rforged",
	} {
		if !strings.Contains(deletionNormalized, text) {
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
	if empty.exitCode != 0 || empty.stderr != "" ||
		!strings.Contains(humanFields(empty.stdout), "Deleted: area 8 Empty") ||
		strings.Count(empty.stdout, "\n") != 1 {
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
		{name: "invalid tag ID", args: []string{"area", "tag", "0", "name", "--json"}},
		{name: "invalid untag ID", args: []string{"area", "untag", "nope", "name", "--json"}},
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
