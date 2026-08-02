package e2e

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestAreaContainmentAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "area-containment", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	home := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	work := decodeAreaRow(t, runJSON("areas", "add", "Work"))
	project := decodeProject(t, runJSON("projects", "add", "Kitchen reno", "--area", fmt.Sprint(home.ID)))
	workAnchor := decodeProject(t, runJSON("projects", "add", "Work anchor", "--area", fmt.Sprint(work.ID)))
	loose := decodeTask(t, runJSON("add", "Change furnace filter", "--area", fmt.Sprint(home.ID)))
	projectTask := decodeTask(t, runJSON("add", "Get quotes", "--project", fmt.Sprint(project.ID)))
	inboxTask := decodeTask(t, runJSON("add", "Buy milk"))

	areaTasks := decodeTasks(t, runJSON("list", "--area", fmt.Sprint(home.ID)))
	if len(areaTasks) != 1 || areaTasks[0].ID != loose.ID {
		t.Errorf("Home task list = %#v, want only direct task %#v", areaTasks, loose)
	}
	areaProjects := decodeProjects(t, runJSON("projects", "list", "--area", fmt.Sprint(home.ID)))
	if len(areaProjects) != 1 || areaProjects[0].ID != project.ID {
		t.Errorf("Home project list = %#v, want only %#v", areaProjects, project)
	}

	inbox := decodeJSON[[]task.ViewTask](t, runJSON("inbox"), "inbox")
	if len(inbox) != 1 || inbox[0].ID != inboxTask.ID || inbox[0].ProjectTitle != nil ||
		inbox[0].GoverningAreaID != nil || inbox[0].GoverningAreaTitle != nil {
		t.Errorf("inbox = %#v, want only unenriched uncontained task %#v", inbox, inboxTask)
	}

	available := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available")
	if len(available) != 3 {
		t.Fatalf("available = %#v, want loose, project, and inbox tasks", available)
	}
	availableByID := make(map[int64]task.ViewTask, len(available))
	for _, view := range available {
		availableByID[view.ID] = view
	}
	assertTaskArea(t, availableByID[loose.ID], nil, home)
	assertTaskArea(t, availableByID[projectTask.ID], &project.Title, home)
	uncontained := availableByID[inboxTask.ID]
	if uncontained.ID == 0 || uncontained.ProjectTitle != nil || uncontained.GoverningAreaID != nil ||
		uncontained.GoverningAreaTitle != nil {
		t.Errorf("available inbox task = %#v, want null enrichment", uncontained)
	}

	for _, show := range [][]string{
		{"show", fmt.Sprint(loose.ID)},
		{"project", "show", fmt.Sprint(project.ID)},
	} {
		result := runGSD(t, append(show, "--db", databasePath)...)
		if !hasHumanRow(result.stdout, "Area", fmt.Sprint(home.ID)) {
			t.Errorf("human show %v = %#v, want Area row for Home", show, result)
		}
	}

	for _, args := range [][]string{
		{"add", "Impossible", "--project", fmt.Sprint(project.ID), "--area", fmt.Sprint(home.ID)},
		{"edit", fmt.Sprint(inboxTask.ID), "--project", fmt.Sprint(project.ID), "--area", fmt.Sprint(home.ID)},
		{"list", "--project", fmt.Sprint(project.ID), "--area", fmt.Sprint(home.ID)},
	} {
		assertJSONError(t, runJSON(args...), apperr.InvalidArgument)
	}
	flagConflict := runJSON("edit", fmt.Sprint(inboxTask.ID), "--area", fmt.Sprint(home.ID), "--no-area")
	if flagConflict.exitCode != 2 || flagConflict.stdout != "" || flagConflict.stderr == "" {
		t.Errorf("same-pair area flags = %#v, want stderr-only usage exit 2", flagConflict)
	}
	assertJSONError(t, runJSON("add", "Missing area", "--area", "99"), apperr.NotFound)
	assertJSONError(t, runJSON("list", "--area", "99"), apperr.NotFound)

	movedToArea := decodeTask(t, runJSON("edit", fmt.Sprint(projectTask.ID), "--area", fmt.Sprint(home.ID)))
	if movedToArea.ProjectID != nil || !pointsTo(movedToArea.AreaID, home.ID) || movedToArea.Position != loose.Position+1 {
		t.Errorf("task moved to Home = %#v, want project cleared and append after %#v", movedToArea, loose)
	}
	movedBack := decodeTask(t, runJSON("edit", fmt.Sprint(projectTask.ID), "--project", fmt.Sprint(project.ID)))
	if !pointsTo(movedBack.ProjectID, project.ID) || movedBack.AreaID != nil {
		t.Errorf("task moved back to project = %#v, want area cleared", movedBack)
	}
	looseInProject := decodeTask(t, runJSON("edit", fmt.Sprint(loose.ID), "--project", fmt.Sprint(project.ID)))
	if !pointsTo(looseInProject.ProjectID, project.ID) || looseInProject.AreaID != nil ||
		looseInProject.Position != movedBack.Position+1 {
		t.Errorf("loose task moved to project = %#v, want area cleared and append after %#v", looseInProject, movedBack)
	}
	projectTaskInInbox := decodeTask(t, runJSON("edit", fmt.Sprint(projectTask.ID), "--no-project"))
	if projectTaskInInbox.ProjectID != nil || projectTaskInInbox.AreaID != nil ||
		projectTaskInInbox.Position != inboxTask.Position+1 {
		t.Errorf("project task moved to inbox = %#v, want memberships clear and append after %#v", projectTaskInInbox, inboxTask)
	}

	movedProject := decodeProject(t, runJSON("project", "edit", fmt.Sprint(project.ID), "--area", fmt.Sprint(work.ID)))
	if !pointsTo(movedProject.AreaID, work.ID) || movedProject.Position != workAnchor.Position+1 {
		t.Errorf("project moved to Work = %#v, want append after %#v", movedProject, workAnchor)
	}
	workProjects := decodeProjects(t, runJSON("projects", "list", "--area", fmt.Sprint(work.ID)))
	if len(workProjects) != 2 || workProjects[0].ID != workAnchor.ID || workProjects[1].ID != project.ID {
		t.Errorf("Work project list = %#v, want anchor then reparented project", workProjects)
	}

	ownAreaTask := decodeTask(t, runJSON("edit", fmt.Sprint(inboxTask.ID), "--area", fmt.Sprint(home.ID)))
	for _, id := range []int64{ownAreaTask.ID, looseInProject.ID, projectTaskInInbox.ID} {
		decodeTask(t, runJSON("done", fmt.Sprint(id)))
	}
	logbookResult := runJSON("logbook")
	entries := decodeLogbook(t, logbookResult)
	assertContainmentLogbookShape(t, logbookResult)
	entriesByID := make(map[int64]logbook.Entry)
	for _, entry := range entries {
		if entry.Kind == "task" {
			entriesByID[entry.ID] = entry
		}
	}
	assertLogbookArea(t, entriesByID[ownAreaTask.ID], home)
	inherited := entriesByID[looseInProject.ID]
	assertLogbookArea(t, inherited, work)
	if !pointsToString(inherited.ProjectTitle, project.Title) {
		t.Errorf("inherited logbook entry = %#v, want project title %q", inherited, project.Title)
	}
	entry := entriesByID[projectTaskInInbox.ID]
	if entry.GoverningAreaID != nil || entry.GoverningAreaTitle != nil {
		t.Errorf("uncontained logbook entry = %#v, want null governing area", entry)
	}

	standalone := decodeProject(t, runJSON("project", "edit", fmt.Sprint(project.ID), "--no-area"))
	if standalone.AreaID != nil {
		t.Errorf("standalone project = %#v, want area cleared", standalone)
	}
}

func TestAreaArchiveAndDeletionAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "area-archive-delete", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	home := decodeAreaRow(t, runJSON("areas", "add", "Home"))
	work := decodeAreaRow(t, runJSON("areas", "add", "Work"))
	homeLoose := decodeTask(t, runJSON("add", "Change furnace filter", "--area", fmt.Sprint(home.ID)))
	homeProject := decodeProject(t, runJSON(
		"projects", "add", "Kitchen reno", "--area", fmt.Sprint(home.ID),
	))
	projectTask := decodeTask(t, runJSON("add", "Get quotes", "--project", fmt.Sprint(homeProject.ID)))
	deletedWhileArchived := decodeTask(t, runJSON(
		"add", "Choose fixtures", "--project", fmt.Sprint(homeProject.ID),
	))
	workLoose := decodeTask(t, runJSON("add", "Submit expenses", "--area", fmt.Sprint(work.ID)))
	inboxTask := decodeTask(t, runJSON("add", "Buy milk"))
	resolvedTask := decodeTask(t, runJSON("add", "Retired reminder", "--area", fmt.Sprint(home.ID)))
	decodeTask(t, runJSON("done", fmt.Sprint(resolvedTask.ID)))
	resolvedProject := decodeProject(t, runJSON(
		"projects", "add", "Finished room", "--area", fmt.Sprint(home.ID),
	))
	decodeProjectResolution(t, runJSON("project", "done", fmt.Sprint(resolvedProject.ID)))

	availableBeforeArchive := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available")
	if len(availableBeforeArchive) != 5 {
		t.Fatalf("available before archive = %#v, want five open tasks", availableBeforeArchive)
	}
	archiveResult := runGSD(
		t,
		"area", "archive", fmt.Sprint(home.ID), "--db", databasePath,
	)
	if archiveResult.exitCode != 0 || archiveResult.stderr != "" ||
		archiveResult.stdout != fmt.Sprintf("Archived: area %d  Home\n", home.ID) {
		t.Fatalf("human archive = %#v, want concise archive mutation", archiveResult)
	}
	archivedHome := decodeAreaRow(t, runJSON("area", "show", fmt.Sprint(home.ID)))
	if archivedHome.ArchivedAt == nil || archivedHome.Position != home.Position {
		t.Errorf("archived Home = %#v, want archived with original position %d", archivedHome, home.Position)
	}

	availableWhileArchived := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available")
	if len(availableWhileArchived) != 2 || availableWhileArchived[0].ID != workLoose.ID ||
		availableWhileArchived[1].ID != inboxTask.ID {
		t.Errorf(
			"available while Home archived = %#v, want only Work and inbox tasks",
			availableWhileArchived,
		)
	}
	directList := decodeTasks(t, runJSON("list", "--area", fmt.Sprint(home.ID)))
	if len(directList) != 1 || directList[0].ID != homeLoose.ID {
		t.Errorf("archived Home direct list = %#v, want loose open task %#v", directList, homeLoose)
	}
	activeAreas := decodeAreaRows(t, runJSON("areas", "list"))
	archivedAreas := decodeAreaRows(t, runJSON("areas", "list", "--archived"))
	allAreas := decodeAreaRows(t, runJSON("areas", "list", "--all"))
	if len(activeAreas) != 1 || activeAreas[0].ID != work.ID ||
		len(archivedAreas) != 1 || archivedAreas[0].ID != home.ID ||
		len(allAreas) != 2 || allAreas[0].ID != home.ID || allAreas[1].ID != work.ID {
		t.Errorf(
			"area partitions active/archived/all = %#v/%#v/%#v, want Work/Home/Home then Work",
			activeAreas,
			archivedAreas,
			allAreas,
		)
	}
	for _, args := range [][]string{
		{"add", "Late idea", "--area", fmt.Sprint(home.ID)},
		{"edit", fmt.Sprint(homeLoose.ID), "--area", fmt.Sprint(work.ID)},
		{"edit", fmt.Sprint(workLoose.ID), "--area", fmt.Sprint(home.ID)},
		{"done", fmt.Sprint(projectTask.ID)},
		{"project", "done", fmt.Sprint(homeProject.ID)},
	} {
		assertJSONError(t, runJSON(args...), apperr.Conflict)
	}
	guardGuidance := runGSD(
		t,
		"done", fmt.Sprint(projectTask.ID), "--db", databasePath,
	)
	if guardGuidance.exitCode != 1 || guardGuidance.stdout != "" ||
		!strings.Contains(guardGuidance.stderr, "area unarchive") ||
		!strings.Contains(guardGuidance.stderr, fmt.Sprint(home.ID)) {
		t.Errorf("human archived guard = %#v, want unarchive-first guidance", guardGuidance)
	}

	editedTask := decodeTask(t, runJSON(
		"edit", fmt.Sprint(projectTask.ID), "--title", "Get contractor quotes",
	))
	if editedTask.Title != "Get contractor quotes" || editedTask.Position != projectTask.Position {
		t.Errorf("content-edited archived task = %#v, want title change without movement", editedTask)
	}
	deletedTask := decodeTask(t, runJSON("delete", fmt.Sprint(deletedWhileArchived.ID)))
	if deletedTask.ID != deletedWhileArchived.ID {
		t.Errorf("deleted archived-area task = %#v, want %#v", deletedTask, deletedWhileArchived)
	}

	unarchivedHome := decodeAreaRow(t, runJSON("area", "unarchive", fmt.Sprint(home.ID)))
	if unarchivedHome.ArchivedAt != nil || unarchivedHome.Position != home.Position {
		t.Errorf("unarchived Home = %#v, want active at original position %d", unarchivedHome, home.Position)
	}
	assertJSONError(
		t,
		runJSON("area", "unarchive", fmt.Sprint(home.ID)),
		apperr.Conflict,
	)
	restoredAvailable := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available")
	if len(restoredAvailable) != 4 {
		t.Fatalf("available after unarchive = %#v, want four remaining open tasks", restoredAvailable)
	}
	wantRestoredIDs := []int64{homeLoose.ID, editedTask.ID, workLoose.ID, inboxTask.ID}
	for index, wantID := range wantRestoredIDs {
		if restoredAvailable[index].ID != wantID {
			t.Errorf("available after unarchive = %#v, want IDs %v", restoredAvailable, wantRestoredIDs)
			break
		}
	}
	if restoredAvailable[0].Position != homeLoose.Position ||
		restoredAvailable[1].Position != projectTask.Position ||
		restoredAvailable[1].Title != editedTask.Title {
		t.Errorf("restored governed tasks = %#v, want original positions and edited title", restoredAvailable[:2])
	}
	assertJSONError(t, runJSON("area", "delete", fmt.Sprint(home.ID)), apperr.Conflict)

	empty := decodeAreaRow(t, runJSON("areas", "add", "Empty"))
	emptyDeletion := decodeAreaRow(t, runJSON("area", "delete", fmt.Sprint(empty.ID)))
	if emptyDeletion.ID != empty.ID {
		t.Errorf("empty area deletion = %#v, want deleted Empty area %#v", emptyDeletion, empty)
	}
	assertJSONError(t, runJSON("area", "show", fmt.Sprint(empty.ID)), apperr.NotFound)

	deletionResult := runJSON("area", "delete", fmt.Sprint(home.ID), "--recursive")
	deletion := decodeJSON[area.Deletion](t, deletionResult, "recursive area deletion")
	if deletion.Area.ID != home.ID || len(deletion.DeletedProjects) != 2 || len(deletion.DeletedTasks) != 3 {
		t.Errorf("recursive deletion = %#v, want Home, two projects, and three tasks", deletion)
	}
	assertJSONError(t, runJSON("area", "show", fmt.Sprint(home.ID)), apperr.NotFound)

	narrated := decodeAreaRow(t, runJSON("areas", "add", "Narrated"))
	narratedProject := decodeProject(t, runJSON(
		"projects", "add", "Narrated project", "--area", fmt.Sprint(narrated.ID),
	))
	decodeTask(t, runJSON("add", "Narrated project task", "--project", fmt.Sprint(narratedProject.ID)))
	decodeTask(t, runJSON("add", "Narrated loose task", "--area", fmt.Sprint(narrated.ID)))
	narration := runGSD(
		t,
		"area", "delete", fmt.Sprint(narrated.ID), "--recursive", "--db", databasePath,
	)
	for _, want := range []string{
		fmt.Sprintf("Deleted: area %d  Narrated", narrated.ID),
		"Deleted 1 project:",
		"Narrated project",
		"Deleted 2 tasks:",
		"Narrated project task",
		"Narrated loose task",
	} {
		if !strings.Contains(narration.stdout, want) {
			t.Errorf("human recursive deletion = %#v, want %q", narration, want)
		}
	}
	if narration.exitCode != 0 || narration.stderr != "" || strings.Contains(narration.stdout, "\x1b[") {
		t.Errorf("human recursive deletion = %#v, want plain stdout-only success", narration)
	}
}

func assertContainmentLogbookShape(t *testing.T, result processResult) {
	t.Helper()
	fields := []string{
		"kind", "id", "title", "status", "resolved_at", "project_title",
		"governing_area_id", "governing_area_title", "tags",
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(result.stdout), &rows); err != nil {
		t.Fatalf("decode logbook JSON fields: %v", err)
	}
	for _, row := range rows {
		if len(row) != len(fields) {
			t.Fatalf("logbook JSON fields = %v, want exactly %v", row, fields)
		}
		for _, field := range fields {
			if _, ok := row[field]; !ok {
				t.Fatalf("logbook JSON fields = %v, want field %q", row, field)
			}
		}
	}
}

func assertTaskArea(t *testing.T, got task.ViewTask, projectTitle *string, area areaRow) {
	t.Helper()
	if (projectTitle == nil && got.ProjectTitle != nil) ||
		(projectTitle != nil && !pointsToString(got.ProjectTitle, *projectTitle)) ||
		!pointsTo(got.GoverningAreaID, area.ID) || !pointsToString(got.GoverningAreaTitle, area.Title) {
		t.Errorf("available task = %#v, want project %v and governing area %#v", got, projectTitle, area)
	}
}

func assertLogbookArea(t *testing.T, got logbook.Entry, area areaRow) {
	t.Helper()
	if !pointsTo(got.GoverningAreaID, area.ID) || !pointsToString(got.GoverningAreaTitle, area.Title) {
		t.Errorf("logbook entry = %#v, want governing area %#v", got, area)
	}
}

func pointsTo(got *int64, want int64) bool {
	return got != nil && *got == want
}

func pointsToString(got *string, want string) bool {
	return got != nil && *got == want
}

func hasHumanRow(output string, label string, value string) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == label && fields[1] == value {
			return true
		}
	}
	return false
}
