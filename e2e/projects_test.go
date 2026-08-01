package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestProjectContainmentWorkflow(t *testing.T) {
	databasePath := filepath.Join(workDir, "projects", "gsd.db")

	kitchen := decodeProject(t, runGSD(
		t,
		"projects",
		"add",
		"Kitchen reno",
		"--db",
		databasePath,
		"--json",
	))
	if kitchen.ID != 1 || kitchen.Title != "Kitchen reno" || kitchen.Status != "open" || kitchen.Position != 0 {
		t.Errorf("kitchen project = %#v, want first open project", kitchen)
	}

	quotes := decodeTask(t, runGSD(
		t,
		"add",
		"Get quotes",
		"--project",
		fmt.Sprint(kitchen.ID),
		"--db",
		databasePath,
		"--json",
	))
	tiles := decodeTask(t, runGSD(
		t,
		"add",
		"Pick tiles",
		"--project",
		fmt.Sprint(kitchen.ID),
		"--db",
		databasePath,
		"--json",
	))
	loose := decodeTask(t, runGSD(t, "add", "Buy milk", "--db", databasePath, "--json"))
	if !hasProject(quotes, kitchen.ID) || quotes.Position != 0 ||
		!hasProject(tiles, kitchen.ID) || tiles.Position != 1 {
		t.Errorf("contained tasks = %#v/%#v, want project-scoped positions 0 and 1", quotes, tiles)
	}
	if loose.ProjectID != nil || loose.Position != 0 {
		t.Errorf("loose task = %#v, want inbox position 0", loose)
	}

	listed := decodeTasks(t, runGSD(
		t,
		"list",
		"--project",
		fmt.Sprint(kitchen.ID),
		"--db",
		databasePath,
		"--json",
	))
	if len(listed) != 2 || listed[0].ID != quotes.ID || listed[1].ID != tiles.ID {
		t.Errorf("project tasks = %#v, want quotes then tiles", listed)
	}
	inbox := decodeTasks(t, runGSD(t, "inbox", "--db", databasePath, "--json"))
	if len(inbox) != 1 || inbox[0].ID != loose.ID {
		t.Errorf("inbox = %#v, want only loose task", inbox)
	}

	bathroom := decodeProject(t, runGSD(
		t,
		"projects",
		"add",
		"Bathroom",
		"--db",
		databasePath,
		"--json",
	))
	projects := decodeProjects(t, runGSD(t, "projects", "list", "--db", databasePath, "--json"))
	if len(projects) != 2 || projects[0].ID != kitchen.ID || projects[1].ID != bathroom.ID {
		t.Errorf("projects = %#v, want kitchen then bathroom", projects)
	}

	bathroomFirst := decodeTask(t, runGSD(
		t,
		"add",
		"Measure walls",
		"--project",
		fmt.Sprint(bathroom.ID),
		"--db",
		databasePath,
		"--json",
	))
	moved := decodeTask(t, runGSD(
		t,
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(bathroom.ID),
		"--db",
		databasePath,
		"--json",
	))
	if !hasProject(moved, bathroom.ID) || moved.Position != bathroomFirst.Position+1 {
		t.Errorf("moved task = %#v, want appended after %#v", moved, bathroomFirst)
	}
	restated := decodeTask(t, runGSD(
		t,
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(bathroom.ID),
		"--db",
		databasePath,
		"--json",
	))
	if !hasProject(restated, bathroom.ID) || restated.Position != moved.Position {
		t.Errorf("same-container edit = %#v, want membership and position unchanged from %#v", restated, moved)
	}

	returned := decodeTask(t, runGSD(
		t,
		"edit",
		fmt.Sprint(tiles.ID),
		"--no-project",
		"--db",
		databasePath,
		"--json",
	))
	if returned.ProjectID != nil || returned.Position != loose.Position+1 {
		t.Errorf("returned task = %#v, want appended after inbox task %#v", returned, loose)
	}

	note := "Budget: 20k"
	editedProject := decodeProject(t, runGSD(
		t,
		"project",
		"edit",
		fmt.Sprint(kitchen.ID),
		"--note",
		note,
		"--db",
		databasePath,
		"--json",
	))
	shownProject := decodeProject(t, runGSD(
		t,
		"project",
		"show",
		fmt.Sprint(kitchen.ID),
		"--db",
		databasePath,
		"--json",
	))
	if editedProject.Note != note || !reflect.DeepEqual(shownProject, editedProject) {
		t.Errorf("edited/shown project = %#v/%#v, want persisted note", editedProject, shownProject)
	}

	assertJSONError(
		t,
		runGSD(t, "add", "missing", "--project", "99", "--db", databasePath, "--json"),
		apperr.NotFound,
	)
	assertJSONError(
		t,
		runGSD(t, "list", "--project", "99", "--db", databasePath, "--json"),
		apperr.NotFound,
	)
	conflict := runGSD(
		t,
		"edit",
		fmt.Sprint(tiles.ID),
		"--project",
		fmt.Sprint(kitchen.ID),
		"--no-project",
		"--db",
		databasePath,
		"--json",
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

func decodeProject(t *testing.T, result processResult) project.Project {
	t.Helper()
	return decodeJSON[project.Project](t, result, "project")
}

func decodeProjects(t *testing.T, result processResult) []project.Project {
	t.Helper()
	return decodeJSON[[]project.Project](t, result, "projects")
}

func hasProject(current task.Task, id int64) bool {
	return current.ProjectID != nil && *current.ProjectID == id
}
