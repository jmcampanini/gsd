package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestProjectContainmentWorkflow(t *testing.T) {
	databasePath := filepath.Join(workDir, "projects", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	kitchen := decodeProject(t, runJSON("projects", "add", "Kitchen reno"))
	if kitchen.ID != 1 || kitchen.Title != "Kitchen reno" || kitchen.Status != "open" || kitchen.Position != 0 {
		t.Errorf("kitchen project = %#v, want first open project", kitchen)
	}

	quotes := decodeTask(t, runJSON("add", "Get quotes", "--project", fmt.Sprint(kitchen.ID)))
	tiles := decodeTask(t, runJSON("add", "Pick tiles", "--project", fmt.Sprint(kitchen.ID)))
	loose := decodeTask(t, runJSON("add", "Buy milk"))
	if !hasProject(quotes, kitchen.ID) || quotes.Position != 0 ||
		!hasProject(tiles, kitchen.ID) || tiles.Position != 1 {
		t.Errorf("contained tasks = %#v/%#v, want project-scoped positions 0 and 1", quotes, tiles)
	}
	if loose.ProjectID != nil || loose.Position != 0 {
		t.Errorf("loose task = %#v, want inbox position 0", loose)
	}

	listed := decodeTasks(t, runJSON("list", "--project", fmt.Sprint(kitchen.ID)))
	if len(listed) != 2 || listed[0].ID != quotes.ID || listed[1].ID != tiles.ID {
		t.Errorf("project tasks = %#v, want quotes then tiles", listed)
	}
	inbox := decodeTasks(t, runJSON("inbox"))
	if len(inbox) != 1 || inbox[0].ID != loose.ID {
		t.Errorf("inbox = %#v, want only loose task", inbox)
	}

	bathroom := decodeProject(t, runJSON("projects", "add", "Bathroom"))
	projects := decodeProjects(t, runJSON("projects", "list"))
	if len(projects) != 2 || projects[0].ID != kitchen.ID || projects[1].ID != bathroom.ID {
		t.Errorf("projects = %#v, want kitchen then bathroom", projects)
	}

	bathroomFirst := decodeTask(t, runJSON(
		"add",
		"Measure walls",
		"--project",
		fmt.Sprint(bathroom.ID),
	))
	moved := decodeTaskEdition(t, runJSON(
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(bathroom.ID),
	)).Task
	if !hasProject(moved, bathroom.ID) || moved.Position != bathroomFirst.Position+1 {
		t.Errorf("moved task = %#v, want appended after %#v", moved, bathroomFirst)
	}
	restated := decodeTaskEdition(t, runJSON(
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(bathroom.ID),
	)).Task
	if !hasProject(restated, bathroom.ID) || restated.Position != moved.Position {
		t.Errorf("same-container edit = %#v, want membership and position unchanged from %#v", restated, moved)
	}

	returned := decodeTaskEdition(t, runJSON("edit", fmt.Sprint(tiles.ID), "--no-project")).Task
	if returned.ProjectID != nil || returned.Position != loose.Position+1 {
		t.Errorf("returned task = %#v, want appended after inbox task %#v", returned, loose)
	}

	note := "Budget: 20k"
	editedProject := decodeProject(t, runJSON(
		"project",
		"edit",
		fmt.Sprint(kitchen.ID),
		"--note",
		note,
	))
	shownProject := decodeProject(t, runJSON("project", "show", fmt.Sprint(kitchen.ID)))
	if editedProject.Note != note || !reflect.DeepEqual(shownProject, editedProject) {
		t.Errorf("edited/shown project = %#v/%#v, want persisted note", editedProject, shownProject)
	}

	assertJSONError(t, runJSON("add", "missing", "--project", "99"), apperr.NotFound)
	assertJSONError(t, runJSON("list", "--project", "99"), apperr.NotFound)
	conflict := runJSON(
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(kitchen.ID),
		"--no-project",
	)
	if conflict.exitCode != 2 || conflict.stdout != "" || conflict.stderr == "" {
		t.Errorf("membership flag conflict = %#v, want stderr-only usage error", conflict)
	}

	for index, noun := range []string{"projects", "project"} {
		unusedPath := filepath.Join(workDir, fmt.Sprintf("bare-project-%d.db", index))
		result := runGSD(t, noun, "--db", unusedPath)
		if result.exitCode != 2 || result.stdout != "" || result.stderr == "" {
			t.Errorf("bare %s = %#v, want stderr-only usage error", noun, result)
		}
		if _, err := os.Stat(unusedPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("bare %s database stat error = %v, want not exist", noun, err)
		}
	}
}

func TestProjectLifecycleWorkflow(t *testing.T) {
	databasePath := filepath.Join(workDir, "project-lifecycle", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	launch := decodeProject(t, runJSON("projects", "add", "Launch"))
	completed := decodeTask(t, runJSON(
		"add",
		"Write announcement",
		"--project",
		fmt.Sprint(launch.ID),
	))
	firstOpen := decodeTask(t, runJSON(
		"add",
		"Publish release",
		"--project",
		fmt.Sprint(launch.ID),
	))
	secondOpen := decodeTask(t, runJSON(
		"add",
		"Notify customers",
		"--project",
		fmt.Sprint(launch.ID),
	))
	completed = decodeTask(t, runJSON("done", fmt.Sprint(completed.ID)))
	if completed.Status != "done" || completed.DoneAt == nil {
		t.Fatalf("completed task = %#v, want done task with resolution timestamp", completed)
	}

	resolved := decodeProjectResolution(t, runJSON(
		"project",
		"done",
		fmt.Sprint(launch.ID),
	))
	if resolved.Project.ID != launch.ID || resolved.Project.Status != "done" ||
		resolved.Project.DoneAt == nil || resolved.Project.CancelledAt != nil {
		t.Fatalf("resolved project = %#v, want done launch project", resolved.Project)
	}
	if len(resolved.CancelledTasks) != 2 ||
		resolved.CancelledTasks[0].ID != firstOpen.ID ||
		resolved.CancelledTasks[1].ID != secondOpen.ID {
		t.Fatalf(
			"cancelled tasks = %#v, want the two open tasks in position order",
			resolved.CancelledTasks,
		)
	}
	for _, cancelled := range resolved.CancelledTasks {
		if cancelled.Status != "cancelled" || cancelled.CancelledAt == nil ||
			*cancelled.CancelledAt != *resolved.Project.DoneAt || cancelled.DoneAt != nil {
			t.Errorf(
				"cascade task = %#v, want cancellation at project resolution timestamp %q",
				cancelled,
				*resolved.Project.DoneAt,
			)
		}
	}
	persistedCompleted := decodeTask(t, runJSON("show", fmt.Sprint(completed.ID)))
	if !reflect.DeepEqual(persistedCompleted, completed) {
		t.Errorf("pre-completed task = %#v, want untouched %#v", persistedCompleted, completed)
	}

	entries := decodeLogbook(t, runJSON("logbook"))
	if len(entries) != 4 {
		t.Fatalf("logbook entries = %#v, want exactly four resolved entries", entries)
	}
	if entries[0].Kind != "project" || entries[0].ID != launch.ID ||
		entries[0].Title != launch.Title || entries[0].Status != "done" ||
		entries[0].ResolvedAt != *resolved.Project.DoneAt || entries[0].ProjectTitle != nil {
		t.Errorf("logbook project entry = %#v, want resolved launch without project title", entries[0])
	}
	for index, want := range []task.Task{secondOpen, firstOpen} {
		entry := entries[index+1]
		if entry.Kind != "task" || entry.ID != want.ID || entry.Title != want.Title ||
			entry.Status != "cancelled" || entry.ResolvedAt != *resolved.Project.DoneAt ||
			entry.ProjectTitle == nil || *entry.ProjectTitle != launch.Title {
			t.Errorf(
				"logbook cascade entry %d = %#v, want cancelled task %#v at project resolution",
				index,
				entry,
				want,
			)
		}
	}
	if entries[3].Kind != "task" || entries[3].ID != completed.ID ||
		entries[3].Title != completed.Title || entries[3].Status != "done" ||
		entries[3].ResolvedAt != *completed.DoneAt || entries[3].ProjectTitle == nil ||
		*entries[3].ProjectTitle != launch.Title {
		t.Errorf("logbook completed entry = %#v, want independently completed task", entries[3])
	}

	assertJSONError(
		t,
		runJSON("project", "done", fmt.Sprint(launch.ID)),
		apperr.Conflict,
	)
	assertJSONError(t, runJSON("reopen", fmt.Sprint(firstOpen.ID)), apperr.Conflict)
	assertJSONError(
		t,
		runJSON("add", "Late idea", "--project", fmt.Sprint(launch.ID)),
		apperr.Conflict,
	)

	edited := decodeTask(t, runJSON(
		"edit",
		fmt.Sprint(firstOpen.ID),
		"--title",
		"Publish final release",
	))
	if edited.Title != "Publish final release" || edited.Status != "cancelled" ||
		!reflect.DeepEqual(edited.CancelledAt, resolved.CancelledTasks[0].CancelledAt) {
		t.Errorf("edited resolved-project task = %#v, want content-only edit", edited)
	}

	reopenedProject := decodeProject(t, runJSON(
		"project",
		"reopen",
		fmt.Sprint(launch.ID),
	))
	if reopenedProject.Status != "open" || reopenedProject.DoneAt != nil ||
		reopenedProject.CancelledAt != nil {
		t.Errorf("reopened project = %#v, want open project", reopenedProject)
	}
	persistedFirst := decodeTask(t, runJSON("show", fmt.Sprint(firstOpen.ID)))
	persistedSecond := decodeTask(t, runJSON("show", fmt.Sprint(secondOpen.ID)))
	if persistedFirst.Status != "cancelled" || persistedSecond.Status != "cancelled" ||
		!reflect.DeepEqual(persistedFirst.CancelledAt, edited.CancelledAt) ||
		!reflect.DeepEqual(persistedSecond, resolved.CancelledTasks[1]) {
		t.Errorf(
			"tasks after project reopen = %#v/%#v, want cascade left intact",
			persistedFirst,
			persistedSecond,
		)
	}

	reopenedTask := decodeTask(t, runJSON("reopen", fmt.Sprint(firstOpen.ID)))
	if reopenedTask.Status != "open" || reopenedTask.DoneAt != nil ||
		reopenedTask.CancelledAt != nil {
		t.Fatalf("reopened task = %#v, want open task", reopenedTask)
	}
	recompleted := decodeProjectResolution(t, runJSON(
		"project",
		"done",
		fmt.Sprint(launch.ID),
	))
	if recompleted.Project.Status != "done" || recompleted.Project.DoneAt == nil ||
		len(recompleted.CancelledTasks) != 1 ||
		recompleted.CancelledTasks[0].ID != reopenedTask.ID ||
		recompleted.CancelledTasks[0].CancelledAt == nil ||
		*recompleted.CancelledTasks[0].CancelledAt != *recompleted.Project.DoneAt {
		t.Errorf("recompleted resolution = %#v, want reopened task recancelled", recompleted)
	}

	decodeProject(t, runJSON("project", "reopen", fmt.Sprint(launch.ID)))
	zeroCascade := decodeProjectResolution(t, runJSON(
		"project",
		"done",
		fmt.Sprint(launch.ID),
	))
	if zeroCascade.Project.Status != "done" || zeroCascade.CancelledTasks == nil ||
		len(zeroCascade.CancelledTasks) != 0 {
		t.Errorf("zero-task resolution = %#v, want done project and empty cancellation array", zeroCascade)
	}

	empty := decodeProject(t, runJSON("projects", "add", "Empty project"))
	deletedEmpty := decodeProject(t, runJSON("project", "delete", fmt.Sprint(empty.ID)))
	if !reflect.DeepEqual(deletedEmpty, empty) {
		t.Errorf("deleted empty project = %#v, want snapshot %#v", deletedEmpty, empty)
	}
	assertJSONError(
		t,
		runJSON("project", "show", fmt.Sprint(empty.ID)),
		apperr.NotFound,
	)

	abandoned := decodeProject(t, runJSON("projects", "add", "Abandoned"))
	abandonedTask := decodeTask(t, runJSON(
		"add",
		"Remove preview environment",
		"--project",
		fmt.Sprint(abandoned.ID),
	))
	cancelled := decodeProjectResolution(t, runJSON(
		"project",
		"cancel",
		fmt.Sprint(abandoned.ID),
	))
	if cancelled.Project.Status != "cancelled" || cancelled.Project.CancelledAt == nil ||
		cancelled.Project.DoneAt != nil || len(cancelled.CancelledTasks) != 1 ||
		cancelled.CancelledTasks[0].ID != abandonedTask.ID ||
		cancelled.CancelledTasks[0].CancelledAt == nil ||
		*cancelled.CancelledTasks[0].CancelledAt != *cancelled.Project.CancelledAt {
		t.Fatalf("cancelled resolution = %#v, want project and task cancelled together", cancelled)
	}
	assertJSONError(
		t,
		runJSON("project", "delete", fmt.Sprint(abandoned.ID)),
		apperr.Conflict,
	)

	deleted := decodeProjectDeletion(t, runJSON(
		"project",
		"delete",
		fmt.Sprint(abandoned.ID),
		"--recursive",
	))
	if !reflect.DeepEqual(deleted.Project, cancelled.Project) ||
		!reflect.DeepEqual(deleted.DeletedTasks, cancelled.CancelledTasks) {
		t.Errorf("recursive deletion = %#v, want cancelled project and its task", deleted)
	}
	assertJSONError(
		t,
		runJSON("project", "show", fmt.Sprint(abandoned.ID)),
		apperr.NotFound,
	)
	assertJSONError(t, runJSON("show", fmt.Sprint(abandonedTask.ID)), apperr.NotFound)
}

func decodeLogbook(t *testing.T, result processResult) []logbook.Entry {
	t.Helper()
	return decodeJSON[[]logbook.Entry](t, result, "logbook")
}

func decodeProject(t *testing.T, result processResult) project.Project {
	t.Helper()
	return decodeJSON[project.Project](t, result, "project")
}

func decodeProjects(t *testing.T, result processResult) []project.Project {
	t.Helper()
	return decodeJSON[[]project.Project](t, result, "projects")
}

func decodeProjectResolution(t *testing.T, result processResult) project.Resolution {
	t.Helper()
	return decodeJSON[project.Resolution](t, result, "project resolution")
}

func decodeProjectDeletion(t *testing.T, result processResult) project.Deletion {
	t.Helper()
	return decodeJSON[project.Deletion](t, result, "project deletion")
}

func hasProject(current task.Task, id int64) bool {
	return current.ProjectID != nil && *current.ProjectID == id
}
