package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestReorderWorkflowAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "reorder", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	milestone := decodeTagRow(t, runJSON("tags", "add", "milestone"))
	home := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	work := decodeAreaRow(t, runJSON("areas", "add", "Work"))
	errands := decodeAreaRow(t, runJSON("areas", "add", "Errands", "--tag", milestone.Title))

	taskProject := decodeProject(t, runJSON("projects", "add", "Task container"))
	standaloneReference := decodeProject(t, runJSON("projects", "add", "Standalone reference"))
	projectDone := decodeProject(t, runJSON("projects", "add", "Finished project", "--area", fmt.Sprint(home.ID)))
	projectCancelled := decodeProject(t, runJSON(
		"projects", "add", "Cancelled project", "--area", fmt.Sprint(home.ID), "--tag", milestone.Title,
	))
	projectOpen := decodeProject(t, runJSON("projects", "add", "Open project", "--area", fmt.Sprint(home.ID)))

	taskDone := decodeTask(t, runJSON("add", "Finished task", "--project", fmt.Sprint(taskProject.ID)))
	taskCancelled := decodeTask(t, runJSON(
		"add", "Cancelled task", "--project", fmt.Sprint(taskProject.ID), "--tag", milestone.Title,
	))
	taskOpen := decodeTask(t, runJSON("add", "Open task", "--project", fmt.Sprint(taskProject.ID)))
	inboxTask := decodeTask(t, runJSON("add", "Inbox task"))
	areaTask := decodeTask(t, runJSON("add", "Area task", "--area", fmt.Sprint(home.ID)))
	mixedFirst := decodeTask(t, runJSON("add", "Mixed first", "--area", fmt.Sprint(work.ID)))
	mixedDone := decodeTask(t, runJSON("add", "Mixed done", "--area", fmt.Sprint(work.ID)))
	mixedThird := decodeTask(t, runJSON("add", "Mixed third", "--area", fmt.Sprint(work.ID)))

	taskDone = decodeTask(t, runJSON("done", fmt.Sprint(taskDone.ID)))
	taskCancelled = decodeTask(t, runJSON("cancel", fmt.Sprint(taskCancelled.ID)))
	projectDone = decodeProjectResolution(t, runJSON("project", "done", fmt.Sprint(projectDone.ID))).Project
	projectCancelled = decodeProjectResolution(t, runJSON(
		"project", "cancel", fmt.Sprint(projectCancelled.ID),
	)).Project
	errands = decodeAreaRow(t, runJSON("area", "archive", fmt.Sprint(errands.ID)))

	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskDone.ID, taskCancelled.ID, taskOpen.ID})
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectDone.ID, projectCancelled.ID, projectOpen.ID})
	assertAreaOrder(t, runJSON, []int64{home.ID, work.ID, errands.ID})

	taskFirstResult := runJSON("reorder", fmt.Sprint(taskCancelled.ID), "--first")
	taskFirst := decodeTask(t, taskFirstResult)
	assertReorderObjectShape(t, taskFirstResult, []string{
		"id", "project_id", "area_id", "title", "note", "defer_until", "due_on", "done_at",
		"cancelled_at", "status", "position", "created_at", "updated_at", "tags",
	})
	if taskFirst.ID != taskCancelled.ID || taskFirst.Status != "cancelled" || taskFirst.Position != 0 ||
		!slices.Equal(taskFirst.Tags, []string{milestone.Title}) {
		t.Errorf("first reordered task = %#v, want tagged cancelled task at position 0", taskFirst)
	}
	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskCancelled.ID, taskDone.ID, taskOpen.ID})

	taskLast := decodeTask(t, runJSON("reorder", fmt.Sprint(taskDone.ID), "--last"))
	if taskLast.Status != "done" || taskLast.Position != 2 {
		t.Errorf("last reordered task = %#v, want done task at position 2", taskLast)
	}
	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskCancelled.ID, taskOpen.ID, taskDone.ID})

	taskBefore := decodeTask(t, runJSON(
		"reorder", fmt.Sprint(taskOpen.ID), "--before", fmt.Sprint(taskCancelled.ID),
	))
	if taskBefore.Position != 0 {
		t.Errorf("before reordered task = %#v, want position 0", taskBefore)
	}
	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskOpen.ID, taskCancelled.ID, taskDone.ID})

	taskAfter := decodeTask(t, runJSON(
		"reorder", fmt.Sprint(taskCancelled.ID), "--after", fmt.Sprint(taskDone.ID),
	))
	if taskAfter.Status != "cancelled" || taskAfter.Position != 2 {
		t.Errorf("after reordered task = %#v, want cancelled task at position 2", taskAfter)
	}
	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskOpen.ID, taskDone.ID, taskCancelled.ID})

	projectFirstResult := runJSON("project", "reorder", fmt.Sprint(projectCancelled.ID), "--first")
	projectFirst := decodeProject(t, projectFirstResult)
	assertReorderObjectShape(t, projectFirstResult, []string{
		"id", "area_id", "title", "note", "done_at", "cancelled_at", "status", "position",
		"created_at", "updated_at", "tags",
	})
	if projectFirst.ID != projectCancelled.ID || projectFirst.Status != "cancelled" ||
		projectFirst.Position != 0 || !slices.Equal(projectFirst.Tags, []string{milestone.Title}) {
		t.Errorf("first reordered project = %#v, want tagged cancelled project at position 0", projectFirst)
	}
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectCancelled.ID, projectDone.ID, projectOpen.ID})

	projectLast := decodeProject(t, runJSON("project", "reorder", fmt.Sprint(projectDone.ID), "--last"))
	if projectLast.Status != "done" || projectLast.Position != 2 {
		t.Errorf("last reordered project = %#v, want done project at position 2", projectLast)
	}
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectCancelled.ID, projectOpen.ID, projectDone.ID})

	projectBefore := decodeProject(t, runJSON(
		"project", "reorder", fmt.Sprint(projectOpen.ID), "--before", fmt.Sprint(projectCancelled.ID),
	))
	if projectBefore.Position != 0 {
		t.Errorf("before reordered project = %#v, want position 0", projectBefore)
	}
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectOpen.ID, projectCancelled.ID, projectDone.ID})

	projectAfter := decodeProject(t, runJSON(
		"project", "reorder", fmt.Sprint(projectCancelled.ID), "--after", fmt.Sprint(projectDone.ID),
	))
	if projectAfter.Status != "cancelled" || projectAfter.Position != 2 {
		t.Errorf("after reordered project = %#v, want cancelled project at position 2", projectAfter)
	}
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectOpen.ID, projectDone.ID, projectCancelled.ID})

	areaFirstResult := runJSON("area", "reorder", fmt.Sprint(errands.ID), "--first")
	areaFirst := decodeAreaRow(t, areaFirstResult)
	if areaFirst.ID != errands.ID || areaFirst.ArchivedAt == nil || areaFirst.Position != 0 ||
		!slices.Equal(areaFirst.Tags, []string{milestone.Title}) {
		t.Errorf("first reordered area = %#v, want tagged archived area at position 0", areaFirst)
	}
	assertAreaOrder(t, runJSON, []int64{errands.ID, home.ID, work.ID})

	areaLast := decodeAreaRow(t, runJSON("area", "reorder", fmt.Sprint(home.ID), "--last"))
	if areaLast.Position != 2 {
		t.Errorf("last reordered area = %#v, want position 2", areaLast)
	}
	assertAreaOrder(t, runJSON, []int64{errands.ID, work.ID, home.ID})

	areaBefore := decodeAreaRow(t, runJSON(
		"area", "reorder", fmt.Sprint(work.ID), "--before", fmt.Sprint(errands.ID),
	))
	if areaBefore.Position != 0 {
		t.Errorf("before reordered area = %#v, want position 0", areaBefore)
	}
	assertAreaOrder(t, runJSON, []int64{work.ID, errands.ID, home.ID})

	areaAfter := decodeAreaRow(t, runJSON(
		"area", "reorder", fmt.Sprint(work.ID), "--after", fmt.Sprint(errands.ID),
	))
	if areaAfter.Position != 1 {
		t.Errorf("after reordered area = %#v, want position 1", areaAfter)
	}
	assertAreaOrder(t, runJSON, []int64{errands.ID, work.ID, home.ID})

	assertJSONError(t, runJSON(
		"reorder", fmt.Sprint(taskOpen.ID), "--before", fmt.Sprint(areaTask.ID),
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON(
		"reorder", fmt.Sprint(taskOpen.ID), "--after", fmt.Sprint(taskOpen.ID),
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON("reorder", "99999", "--first"), apperr.NotFound)
	assertTaskContainerOrder(t, runJSON, taskProject.ID, []int64{taskOpen.ID, taskDone.ID, taskCancelled.ID})

	assertJSONError(t, runJSON(
		"project", "reorder", fmt.Sprint(projectOpen.ID), "--before", fmt.Sprint(standaloneReference.ID),
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON(
		"project", "reorder", fmt.Sprint(projectOpen.ID), "--after", fmt.Sprint(projectOpen.ID),
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON(
		"project", "reorder", fmt.Sprint(projectOpen.ID), "--after", "99999",
	), apperr.NotFound)
	assertProjectContainerOrder(t, runJSON, home.ID, []int64{projectOpen.ID, projectDone.ID, projectCancelled.ID})

	assertJSONError(t, runJSON(
		"area", "reorder", fmt.Sprint(home.ID), "--before", fmt.Sprint(home.ID),
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON("area", "reorder", "99999", "--last"), apperr.NotFound)
	assertAreaOrder(t, runJSON, []int64{errands.ID, work.ID, home.ID})

	if inboxTask.ProjectID != nil || inboxTask.AreaID != nil || areaTask.AreaID == nil {
		t.Fatalf("task container seeds = %#v/%#v, want inbox and area task containers", inboxTask, areaTask)
	}

	mixedFourth := decodeTask(t, runJSON("add", "Mixed fourth", "--area", fmt.Sprint(work.ID)))
	mixedDone = decodeTask(t, runJSON("done", fmt.Sprint(mixedDone.ID)))
	mixedMoved := decodeTask(t, runJSON(
		"reorder", fmt.Sprint(mixedFourth.ID), "--before", fmt.Sprint(mixedDone.ID),
	))
	if mixedMoved.Position != 1 {
		t.Errorf("mixed first reorder = %#v, want position 1", mixedMoved)
	}
	assertAreaTaskOrder(t, runJSON, work.ID, []int64{
		mixedFirst.ID, mixedFourth.ID, mixedDone.ID, mixedThird.ID,
	})

	decodeTask(t, runJSON("delete", fmt.Sprint(mixedFirst.ID)))
	mixedDone = decodeTask(t, runJSON("reorder", fmt.Sprint(mixedDone.ID), "--last"))
	if mixedDone.Status != "done" || mixedDone.Position != 2 {
		t.Errorf("mixed reordered done task = %#v, want done at position 2", mixedDone)
	}
	mixedFifth := decodeTask(t, runJSON("add", "Mixed fifth", "--area", fmt.Sprint(work.ID)))
	if mixedFifth.Position != 3 {
		t.Errorf("mixed appended task = %#v, want collision-free position 3", mixedFifth)
	}
	assertAreaTaskOrder(t, runJSON, work.ID, []int64{
		mixedFourth.ID, mixedThird.ID, mixedDone.ID, mixedFifth.ID,
	})
}

func TestReorderPlacementArityDoesNotOpenDatabase(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "task missing", args: []string{"reorder", "1"}},
		{name: "task multiple", args: []string{"reorder", "1", "--first", "--last"}},
		{name: "project missing", args: []string{"project", "reorder", "1"}},
		{name: "project multiple", args: []string{"project", "reorder", "1", "--before", "2", "--after", "3"}},
		{name: "area missing", args: []string{"area", "reorder", "1"}},
		{name: "area multiple", args: []string{"area", "reorder", "1", "--first", "--after", "2"}},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			databasePath := filepath.Join(workDir, "reorder-arity", fmt.Sprintf("%d.db", index))
			args := append(slices.Clone(test.args), "--db", databasePath, "--json")
			result := runGSD(t, args...)
			if result.exitCode != 2 || result.stdout != "" || result.stderr == "" {
				t.Errorf("placement arity result = %#v, want stderr-only exit 2", result)
			}
			if _, err := os.Stat(databasePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("database stat error = %v, want not exist", err)
			}
		})
	}
}

func assertTaskContainerOrder(
	t *testing.T,
	runJSON func(...string) processResult,
	projectID int64,
	want []int64,
) {
	t.Helper()
	rows := decodeTasks(t, runJSON(
		"list", "--project", fmt.Sprint(projectID), "--status", "all",
	))
	assertTaskRowsOrder(t, "project task container", rows, want)
}

func assertAreaTaskOrder(
	t *testing.T,
	runJSON func(...string) processResult,
	areaID int64,
	want []int64,
) {
	t.Helper()
	rows := decodeTasks(t, runJSON(
		"list", "--area", fmt.Sprint(areaID), "--status", "all",
	))
	assertTaskRowsOrder(t, "area task container", rows, want)
}

func assertTaskRowsOrder(t *testing.T, description string, rows []task.Task, want []int64) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("%s rows = %#v, want IDs %v", description, rows, want)
	}
	for index, wantID := range want {
		if rows[index].ID != wantID || rows[index].Position != int64(index) {
			t.Errorf("%s rows = %#v, want IDs %v at contiguous positions", description, rows, want)
			return
		}
	}
}

func assertProjectContainerOrder(
	t *testing.T,
	runJSON func(...string) processResult,
	areaID int64,
	want []int64,
) {
	t.Helper()
	rows := decodeProjects(t, runJSON(
		"projects", "list", "--area", fmt.Sprint(areaID), "--status", "all",
	))
	if len(rows) != len(want) {
		t.Fatalf("area project container rows = %#v, want IDs %v", rows, want)
	}
	for index, wantID := range want {
		if rows[index].ID != wantID || rows[index].Position != int64(index) {
			t.Errorf("area project container rows = %#v, want IDs %v at contiguous positions", rows, want)
			return
		}
	}
}

func assertAreaOrder(t *testing.T, runJSON func(...string) processResult, want []int64) {
	t.Helper()
	rows := decodeAreaRows(t, runJSON("areas", "list", "--all"))
	if len(rows) != len(want) {
		t.Fatalf("area rows = %#v, want IDs %v", rows, want)
	}
	for index, wantID := range want {
		if rows[index].ID != wantID || rows[index].Position != int64(index) {
			t.Errorf("area rows = %#v, want IDs %v at contiguous positions", rows, want)
			return
		}
	}
}

func assertReorderObjectShape(t *testing.T, result processResult, fields []string) {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &object); err != nil {
		t.Fatalf("decode reordered object: %v", err)
	}
	if len(object) != len(fields) {
		t.Fatalf("reordered JSON fields = %v, want exactly %v", object, fields)
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			t.Fatalf("reordered JSON fields = %v, want field %q", object, field)
		}
	}
}
