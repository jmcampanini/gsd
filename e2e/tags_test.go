package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type tagRow struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type listedTagRow struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	UsageCount int64  `json:"usage_count"`
}

type tagDeletion struct {
	Tag      tagRow `json:"tag"`
	Detached int64  `json:"detached"`
}

func TestTagAdministrationAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "tags", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	bare := runGSD(t, "tags", "--db", databasePath)
	if bare.exitCode != 2 || bare.stdout != "" || bare.stderr == "" {
		t.Errorf("bare tags = %#v, want stderr-only usage error", bare)
	}
	if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("bare tags database stat error = %v, want not exist", err)
	}

	errands := decodeTagRow(t, runJSON("tags", "add", "errands"))
	home := decodeTagRow(t, runJSON("tags", "add", "home"))
	if errands.ID != 1 || errands.Title != "errands" || errands.CreatedAt == "" || errands.UpdatedAt == "" {
		t.Errorf("errands tag = %#v, want complete first tag row", errands)
	}
	if home.ID != 2 || home.Title != "home" || home.CreatedAt == "" || home.UpdatedAt == "" {
		t.Errorf("home tag = %#v, want complete second tag row", home)
	}

	listed := decodeListedTagRows(t, runJSON("tags", "list"))
	if len(listed) != 2 || listed[0].ID != errands.ID || listed[0].Title != errands.Title ||
		listed[0].UsageCount != 0 || listed[1].ID != home.ID || listed[1].Title != home.Title ||
		listed[1].UsageCount != 0 {
		t.Errorf("listed tags = %#v, want errands then home with zero usage", listed)
	}

	conflictMessage := decodeTagError(
		t,
		runJSON("tags", "add", "Errands"),
		apperr.Conflict,
	)
	if !strings.Contains(conflictMessage, errands.Title) {
		t.Errorf("conflict message = %q, want stored spelling %q", conflictMessage, errands.Title)
	}

	renamed := decodeTagRow(t, runJSON("tags", "rename", "errands", "out-and-about"))
	if renamed.ID != errands.ID || renamed.Title != "out-and-about" || renamed.CreatedAt != errands.CreatedAt {
		t.Errorf("renamed errands = %#v, want in-place rename of %#v", renamed, errands)
	}
	caseRenamed := decodeTagRow(t, runJSON("tags", "rename", "home", "HOME"))
	if caseRenamed.ID != home.ID || caseRenamed.Title != "HOME" || caseRenamed.CreatedAt != home.CreatedAt {
		t.Errorf("case-renamed home = %#v, want in-place rename of %#v", caseRenamed, home)
	}

	listed = decodeListedTagRows(t, runJSON("tags", "list"))
	if len(listed) != 2 || listed[0].ID != caseRenamed.ID || listed[0].Title != caseRenamed.Title ||
		listed[0].UsageCount != 0 || listed[1].ID != renamed.ID || listed[1].Title != renamed.Title ||
		listed[1].UsageCount != 0 {
		t.Errorf("listed renamed tags = %#v, want HOME then out-and-about with zero usage", listed)
	}

	deleted := decodeTagDeletion(t, runJSON("tags", "delete", "out-and-about"))
	if deleted.Tag.ID != renamed.ID || deleted.Tag.Title != renamed.Title || deleted.Detached != 0 {
		t.Errorf("deleted tag = %#v, want out-and-about detached from zero items", deleted)
	}
	remaining := decodeListedTagRows(t, runJSON("tags", "list"))
	if len(remaining) != 1 || remaining[0].ID != caseRenamed.ID || remaining[0].Title != caseRenamed.Title {
		t.Errorf("tags after deletion = %#v, want only HOME", remaining)
	}

	decodeTagError(t, runJSON("tags", "rename", "ghost", "x"), apperr.NotFound)
	decodeTagError(t, runJSON("tags", "delete", "ghost"), apperr.NotFound)
	decodeTagError(t, runJSON("tags", "add", ""), apperr.InvalidArgument)
}

func TestTagAttachmentWorkflowAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "tag-attachments", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}
	assertTags := func(description string, got, want []string) {
		t.Helper()
		if got == nil || !slices.Equal(got, want) {
			t.Errorf("%s tags = %#v, want complete array %#v", description, got, want)
		}
	}
	assertUsage := func(want map[string]int64) {
		t.Helper()
		listed := decodeListedTagRows(t, runJSON("tags", "list"))
		if len(listed) != len(want) {
			t.Fatalf("listed tags = %#v, want usage %#v", listed, want)
		}
		for _, current := range listed {
			count, ok := want[current.Title]
			if !ok || current.UsageCount != count {
				t.Errorf("listed tag = %#v, want usage %#v", current, want)
			}
		}
	}

	errands := decodeTagRow(t, runJSON("tags", "add", "errands"))
	home := decodeTagRow(t, runJSON("tags", "add", "home"))
	place := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	todo := decodeTask(t, runJSON("add", "Drop off dry cleaning", "--area", fmt.Sprint(place.ID)))
	kitchen := decodeProject(t, runJSON("projects", "add", "Kitchen reno"))
	assertTags("new task", todo.Tags, []string{})
	assertTags("new project", kitchen.Tags, []string{})
	assertTags("new area", place.Tags, []string{})

	todo = decodeTask(t, runJSON("tag", fmt.Sprint(todo.ID), "ERRANDS"))
	kitchen = decodeProject(t, runJSON("project", "tag", fmt.Sprint(kitchen.ID), "errands"))
	place = decodeAreaRow(t, runJSON("area", "tag", fmt.Sprint(place.ID), "Errands"))
	assertTags("tagged task", todo.Tags, []string{errands.Title})
	assertTags("tagged project", kitchen.Tags, []string{errands.Title})
	assertTags("tagged area", place.Tags, []string{errands.Title})
	assertUsage(map[string]int64{errands.Title: 3, home.Title: 0})

	unknownCommands := [][]string{
		{"tag", fmt.Sprint(todo.ID), "ghost"},
		{"untag", fmt.Sprint(todo.ID), "ghost"},
		{"project", "tag", fmt.Sprint(kitchen.ID), "ghost"},
		{"area", "tag", fmt.Sprint(place.ID), "ghost"},
		{"add", "Rolled back task", "--tag", "ghost"},
		{"list", "--tag", "ghost"},
	}
	for _, args := range unknownCommands {
		assertJSONError(t, runJSON(args...), apperr.NotFound)
	}
	allTasks := decodeTasks(t, runJSON("list", "--status", "all"))
	if len(allTasks) != 1 || allTasks[0].ID != todo.ID {
		t.Errorf("tasks after unknown tagged add = %#v, want only original task %#v", allTasks, todo)
	}

	assertJSONError(
		t,
		runJSON("tag", fmt.Sprint(todo.ID), home.Title, "ghost"),
		apperr.NotFound,
	)
	persistedTodo := decodeTask(t, runJSON("show", fmt.Sprint(todo.ID)))
	assertTags("task after mixed known/unknown attach", persistedTodo.Tags, []string{errands.Title})
	assertUsage(map[string]int64{errands.Title: 3, home.Title: 0})

	duplicate := decodeTask(t, runJSON("tag", fmt.Sprint(todo.ID), "Errands"))
	detachedAbsent := decodeTask(t, runJSON("untag", fmt.Sprint(todo.ID), "HOME"))
	assertTags("task after duplicate attach", duplicate.Tags, []string{errands.Title})
	assertTags("task after absent detach", detachedAbsent.Tags, []string{errands.Title})
	assertUsage(map[string]int64{errands.Title: 3, home.Title: 0})

	taggedArea := decodeAreaRow(t, runJSON(
		"areas", "add", "Tagged area",
		"--tag", "HOME",
		"--tag", "errands",
	))
	taggedProject := decodeProject(t, runJSON(
		"projects", "add", "Tagged project",
		"--area", fmt.Sprint(taggedArea.ID),
		"--tag", "home",
		"--tag", "ERRANDS",
	))
	taggedTask := decodeTask(t, runJSON(
		"add", "Tagged task",
		"--project", fmt.Sprint(taggedProject.ID),
		"--tag", "errands",
		"--tag", "HOME",
	))
	wantBoth := []string{errands.Title, home.Title}
	assertTags("pre-tagged area", taggedArea.Tags, wantBoth)
	assertTags("pre-tagged project", taggedProject.Tags, wantBoth)
	assertTags("pre-tagged task", taggedTask.Tags, wantBoth)
	assertUsage(map[string]int64{errands.Title: 6, home.Title: 3})

	projectFiltered := decodeTasks(t, runJSON(
		"list",
		"--status", "open",
		"--project", fmt.Sprint(taggedProject.ID),
		"--tag", "hOmE",
	))
	areaFiltered := decodeTasks(t, runJSON(
		"list",
		"--status", "open",
		"--area", fmt.Sprint(place.ID),
		"--tag", "ERRANDS",
	))
	if len(projectFiltered) != 1 || projectFiltered[0].ID != taggedTask.ID ||
		len(areaFiltered) != 1 || areaFiltered[0].ID != todo.ID {
		t.Errorf(
			"composed project/area tag filters = %#v/%#v, want tasks %d/%d",
			projectFiltered,
			areaFiltered,
			taggedTask.ID,
			todo.ID,
		)
	} else {
		assertTags("project-and-tag filtered task", projectFiltered[0].Tags, wantBoth)
		assertTags("area-and-tag filtered task", areaFiltered[0].Tags, []string{errands.Title})
	}

	renamed := decodeTagRow(t, runJSON("tags", "rename", "ERRANDS", "out-and-about"))
	if renamed.ID != errands.ID || renamed.Title != "out-and-about" {
		t.Errorf("renamed attached tag = %#v, want ID %d and propagated title", renamed, errands.ID)
	}
	persistedTodo = decodeTask(t, runJSON("show", fmt.Sprint(todo.ID)))
	persistedKitchen := decodeProject(t, runJSON("project", "show", fmt.Sprint(kitchen.ID)))
	persistedPlace := decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(place.ID)))
	persistedTaggedTask := decodeTask(t, runJSON("show", fmt.Sprint(taggedTask.ID)))
	persistedTaggedProject := decodeProject(t, runJSON("project", "show", fmt.Sprint(taggedProject.ID)))
	persistedTaggedArea := decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(taggedArea.ID)))
	assertTags("task after rename", persistedTodo.Tags, []string{renamed.Title})
	assertTags("project after rename", persistedKitchen.Tags, []string{renamed.Title})
	assertTags("area after rename", persistedPlace.Tags, []string{renamed.Title})
	wantRenamedBoth := []string{renamed.Title, home.Title}
	assertTags("pre-tagged task after rename", persistedTaggedTask.Tags, wantRenamedBoth)
	assertTags("pre-tagged project after rename", persistedTaggedProject.Tags, wantRenamedBoth)
	assertTags("pre-tagged area after rename", persistedTaggedArea.Tags, wantRenamedBoth)
	assertUsage(map[string]int64{home.Title: 3, renamed.Title: 6})

	deletedTag := decodeTagDeletion(t, runJSON("tags", "delete", "OUT-AND-ABOUT"))
	if deletedTag.Tag.ID != renamed.ID || deletedTag.Tag.Title != renamed.Title || deletedTag.Detached != 6 {
		t.Errorf("deleted attached tag = %#v, want %#v detached from 6 entities", deletedTag, renamed)
	}
	persistedTodo = decodeTask(t, runJSON("show", fmt.Sprint(todo.ID)))
	persistedKitchen = decodeProject(t, runJSON("project", "show", fmt.Sprint(kitchen.ID)))
	persistedPlace = decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(place.ID)))
	persistedTaggedTask = decodeTask(t, runJSON("show", fmt.Sprint(taggedTask.ID)))
	persistedTaggedProject = decodeProject(t, runJSON("project", "show", fmt.Sprint(taggedProject.ID)))
	persistedTaggedArea = decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(taggedArea.ID)))
	if persistedTodo.ID != todo.ID || persistedKitchen.ID != kitchen.ID || persistedPlace.ID != place.ID {
		t.Errorf(
			"entities after tag deletion = %d/%d/%d, want intact IDs %d/%d/%d",
			persistedTodo.ID,
			persistedKitchen.ID,
			persistedPlace.ID,
			todo.ID,
			kitchen.ID,
			place.ID,
		)
	}
	assertTags("task after tag deletion", persistedTodo.Tags, []string{})
	assertTags("project after tag deletion", persistedKitchen.Tags, []string{})
	assertTags("area after tag deletion", persistedPlace.Tags, []string{})
	assertTags("pre-tagged task after tag deletion", persistedTaggedTask.Tags, []string{home.Title})
	assertTags("pre-tagged project after tag deletion", persistedTaggedProject.Tags, []string{home.Title})
	assertTags("pre-tagged area after tag deletion", persistedTaggedArea.Tags, []string{home.Title})
	assertUsage(map[string]int64{home.Title: 3})

	deletedTask := decodeTask(t, runJSON("delete", fmt.Sprint(taggedTask.ID)))
	if deletedTask.ID != taggedTask.ID {
		t.Errorf("deleted tagged task = %#v, want task %d", deletedTask, taggedTask.ID)
	}
	assertUsage(map[string]int64{home.Title: 2})

	areaChild := decodeTask(t, runJSON("show", fmt.Sprint(todo.ID)))
	projectChild := decodeTask(t, runJSON("add", "Resolved project task", "--project", fmt.Sprint(kitchen.ID)))
	archived := decodeAreaRow(t, runJSON("area", "archive", fmt.Sprint(place.ID)))
	if archived.ArchivedAt == nil {
		t.Errorf("archived area = %#v, want archived timestamp", archived)
	}
	archived = decodeAreaRow(t, runJSON("area", "tag", fmt.Sprint(place.ID), "HOME"))
	areaChild = decodeTask(t, runJSON("tag", fmt.Sprint(areaChild.ID), "home"))
	assertTags("archived area attachment", archived.Tags, []string{home.Title})
	assertTags("task under archived area attachment", areaChild.Tags, []string{home.Title})

	resolution := decodeProjectResolution(t, runJSON("project", "done", fmt.Sprint(kitchen.ID)))
	if resolution.Project.Status != "done" || len(resolution.CancelledTasks) != 1 ||
		resolution.CancelledTasks[0].ID != projectChild.ID {
		t.Errorf("resolved project = %#v, want done project and cancelled child %d", resolution, projectChild.ID)
	}
	resolved := decodeProject(t, runJSON("project", "tag", fmt.Sprint(kitchen.ID), "HOME"))
	projectChild = decodeTask(t, runJSON("tag", fmt.Sprint(projectChild.ID), "home"))
	assertTags("resolved project attachment", resolved.Tags, []string{home.Title})
	assertTags("task under resolved project attachment", projectChild.Tags, []string{home.Title})
	assertUsage(map[string]int64{home.Title: 6})
}

func decodeTagRow(t *testing.T, result processResult) tagRow {
	t.Helper()
	return decodeJSON[tagRow](t, result, "tag")
}

func decodeListedTagRows(t *testing.T, result processResult) []listedTagRow {
	t.Helper()
	return decodeJSON[[]listedTagRow](t, result, "listed tags")
}

func decodeTagDeletion(t *testing.T, result processResult) tagDeletion {
	t.Helper()
	return decodeJSON[tagDeletion](t, result, "tag deletion")
}

func decodeTagError(t *testing.T, result processResult, wantCode apperr.Code) string {
	t.Helper()
	assertJSONError(t, result, wantCode)

	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(result.stderr), &envelope); err != nil {
		t.Fatalf("decode tag error: %v", err)
	}
	return envelope.Error.Message
}
