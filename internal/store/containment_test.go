package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestTaskAddScopesPositionsByFullContainerAndFiltersByContainer(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	firstArea := addStoredArea(t, areas, area.AddFields{Title: "first area"})
	secondArea := addStoredArea(t, areas, area.AddFields{Title: "second area"})
	firstProject := addStoredProject(t, projects, project.AddFields{Title: "first project"})
	secondProject := addStoredProject(t, projects, project.AddFields{Title: "second project"})

	looseFirst := addStoredTask(t, tasks, task.AddFields{Title: "loose first"})
	projectFirst := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "project first"})
	areaFirst := addStoredTask(t, tasks, task.AddFields{AreaID: &firstArea.ID, Title: "area first"})
	looseSecond := addStoredTask(t, tasks, task.AddFields{Title: "loose second"})
	otherAreaFirst := addStoredTask(t, tasks, task.AddFields{AreaID: &secondArea.ID, Title: "other area first"})
	otherProjectFirst := addStoredTask(t, tasks, task.AddFields{ProjectID: &secondProject.ID, Title: "other project first"})
	areaSecond := addStoredTask(t, tasks, task.AddFields{AreaID: &firstArea.ID, Title: "area second"})
	projectSecond := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "project second"})

	positions := []int64{
		looseFirst.Position,
		projectFirst.Position,
		areaFirst.Position,
		looseSecond.Position,
		otherAreaFirst.Position,
		otherProjectFirst.Position,
		areaSecond.Position,
		projectSecond.Position,
	}
	wantPositions := []int64{0, 0, 0, 1, 0, 0, 1, 1}
	if !reflect.DeepEqual(positions, wantPositions) {
		t.Errorf("interleaved positions = %v, want independently scoped %v", positions, wantPositions)
	}
	if projectFirst.ProjectID == nil || *projectFirst.ProjectID != firstProject.ID || projectFirst.AreaID != nil {
		t.Errorf("project task containment = %#v/%#v, want project %d only", projectFirst.ProjectID, projectFirst.AreaID, firstProject.ID)
	}
	if areaFirst.AreaID == nil || *areaFirst.AreaID != firstArea.ID || areaFirst.ProjectID != nil {
		t.Errorf("area task containment = %#v/%#v, want area %d only", areaFirst.ProjectID, areaFirst.AreaID, firstArea.ID)
	}

	filters := []struct {
		name       string
		options    task.ListOptions
		wantTaskID []int64
	}{
		{"project", task.ListOptions{Status: task.ListStatusAll, ProjectID: &firstProject.ID}, []int64{projectFirst.ID, projectSecond.ID}},
		{"area", task.ListOptions{Status: task.ListStatusAll, AreaID: &firstArea.ID}, []int64{areaFirst.ID, areaSecond.ID}},
	}
	for _, filter := range filters {
		listed, err := tasks.List(ctx, filter.options)
		if err != nil {
			t.Fatalf("List(%s) error = %v", filter.name, err)
		}
		listedIDs := make([]int64, len(listed))
		for i := range listed {
			listedIDs[i] = listed[i].ID
		}
		if !reflect.DeepEqual(listedIDs, filter.wantTaskID) {
			t.Errorf("List(%s) IDs = %v, want %v", filter.name, listedIDs, filter.wantTaskID)
		}

		filter.options.Status = task.ListStatusDone
		listed, err = tasks.List(ctx, filter.options)
		if err != nil || len(listed) != 0 {
			t.Errorf("List(existing filtered-empty %s) = %#v, %v; want [], nil", filter.name, listed, err)
		}
	}

	missingID := int64(99)
	for _, filter := range []task.ListOptions{
		{Status: task.ListStatusDone, ProjectID: &missingID},
		{Status: task.ListStatusDone, AreaID: &missingID},
	} {
		if _, err := tasks.List(ctx, filter); errorCode(err) != apperr.NotFound {
			t.Errorf("List(missing container, %#v) error = %v, want not_found", filter, err)
		}
	}
}

func TestTaskAddClassifiesMissingAndResolvedProjects(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	missingProjectID := int64(99)
	if _, err := tasks.Add(
		ctx,
		task.AddFields{ProjectID: &missingProjectID, Title: "missing project"},
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Add(missing project) error = %v, want not_found", err)
	}
	missingAreaID := int64(99)
	if _, err := tasks.Add(
		ctx,
		task.AddFields{AreaID: &missingAreaID, Title: "missing area"},
		"2026-01-01T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Add(missing area) error = %v, want not_found", err)
	}

	resolved, err := projects.Add(
		ctx,
		project.AddFields{Title: "resolved"},
		"2026-01-01T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET cancelled_at = ? WHERE id = ?",
		"2026-01-02T00:00:00.000Z",
		resolved.ID,
	); err != nil {
		t.Fatalf("resolve project fixture: %v", err)
	}
	_, err = tasks.Add(
		ctx,
		task.AddFields{ProjectID: &resolved.ID, Title: "blocked"},
		"2026-01-03T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict {
		t.Errorf("Add(resolved project) error = %v, want conflict", err)
	}

	var taskCount int
	if err := storage.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Errorf("task count = %d, want no inserts from rejected additions", taskCount)
	}
}

func TestTaskAddRejectsArchivedGoverningAreasAtomically(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	directArea := addStoredArea(t, areas, area.AddFields{Title: "direct"})
	inheritedArea := addStoredArea(t, areas, area.AddFields{Title: "inherited"})
	openProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &inheritedArea.ID, Title: "open project"},
	)
	resolvedProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &inheritedArea.ID, Title: "resolved project"},
	)
	if _, err := projects.Resolve(
		ctx,
		resolvedProject.ID,
		project.ExitDone,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(project) error = %v", err)
	}
	archiveStoredAreas(t, storage, directArea.ID, inheritedArea.ID)

	tests := []struct {
		name          string
		fields        task.AddFields
		wantAreaID    int64
		wantProjectID int64
	}{
		{
			name:       "direct area",
			fields:     task.AddFields{AreaID: &directArea.ID, Title: "direct blocked"},
			wantAreaID: directArea.ID,
		},
		{
			name:       "inherited area",
			fields:     task.AddFields{ProjectID: &openProject.ID, Title: "inherited blocked"},
			wantAreaID: inheritedArea.ID,
		},
		{
			name:          "resolved project in archived area",
			fields:        task.AddFields{ProjectID: &resolvedProject.ID, Title: "both blocked"},
			wantAreaID:    inheritedArea.ID,
			wantProjectID: resolvedProject.ID,
		},
	}
	for _, test := range tests {
		_, err := tasks.Add(ctx, test.fields, "2026-01-03T00:00:00.000Z")
		if errorCode(err) != apperr.Conflict {
			t.Errorf("Add(%s) error = %v, want conflict", test.name, err)
			continue
		}
		assertArchivedAreaIDs(t, err, []int64{test.wantAreaID})
		if !strings.Contains(err.Error(), fmt.Sprintf("area %d", test.wantAreaID)) {
			t.Errorf("Add(%s) error = %v, want governing area ID %d", test.name, err, test.wantAreaID)
		}
		if test.wantProjectID != 0 &&
			!strings.Contains(err.Error(), fmt.Sprintf("project %d", test.wantProjectID)) {
			t.Errorf("Add(%s) error = %v, want resolved project ID %d", test.name, err, test.wantProjectID)
		}
	}

	var taskCount int
	if err := storage.database.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&taskCount); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if taskCount != 0 {
		t.Errorf("task count = %d, want rejected additions not persisted", taskCount)
	}
}

func TestTaskEditReparentsBetweenProjectAreaAndInbox(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	containerArea := addStoredArea(t, areas, area.AddFields{Title: "Home"})
	containerProject := addStoredProject(t, projects, project.AddFields{Title: "Kitchen"})

	inboxAnchor := addStoredTask(t, tasks, task.AddFields{Title: "inbox anchor"})
	areaAnchor := addStoredTask(t, tasks, task.AddFields{AreaID: &containerArea.ID, Title: "area anchor"})
	projectAnchor := addStoredTask(t, tasks, task.AddFields{ProjectID: &containerProject.ID, Title: "project anchor"})
	moving := addStoredTask(t, tasks, task.AddFields{ProjectID: &containerProject.ID, Title: "moving"})

	movedToArea, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Area: task.AreaChange{Set: &containerArea.ID}},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(project to area) error = %v", err)
	}
	if movedToArea.ProjectID != nil || movedToArea.AreaID == nil ||
		*movedToArea.AreaID != containerArea.ID || movedToArea.Position != areaAnchor.Position+1 {
		t.Errorf("project-to-area task = %#v, want area-only append", movedToArea)
	}

	redundantProjectClear, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Clear: true}},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(redundant project clear) error = %v", err)
	}
	if !reflect.DeepEqual(redundantProjectClear, movedToArea) {
		t.Errorf("redundant project clear = %#v, want no-op %#v", redundantProjectClear, movedToArea)
	}

	restatedArea, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Area: task.AreaChange{Set: &containerArea.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate area) error = %v", err)
	}
	if !reflect.DeepEqual(restatedArea, movedToArea) {
		t.Errorf("restated area = %#v, want no-op %#v", restatedArea, movedToArea)
	}

	revisedTitle := "revised without movement"
	contentWithRestatedArea, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{
			Area:  task.AreaChange{Set: &containerArea.ID},
			Title: &revisedTitle,
		},
		"2026-01-04T12:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content with restated area) error = %v", err)
	}
	if contentWithRestatedArea.Title != revisedTitle ||
		contentWithRestatedArea.Position != movedToArea.Position ||
		contentWithRestatedArea.UpdatedAt != "2026-01-04T12:00:00.000Z" {
		t.Errorf("content edit with restated area = %#v, want content update without movement", contentWithRestatedArea)
	}

	movedToProject, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &containerProject.ID}},
		"2026-01-05T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(area to project) error = %v", err)
	}
	if movedToProject.ProjectID == nil || *movedToProject.ProjectID != containerProject.ID ||
		movedToProject.AreaID != nil || movedToProject.Position != projectAnchor.Position+1 {
		t.Errorf("area-to-project task = %#v, want project-only append", movedToProject)
	}

	redundantAreaClear, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Area: task.AreaChange{Clear: true}},
		"2026-01-06T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(redundant area clear) error = %v", err)
	}
	if !reflect.DeepEqual(redundantAreaClear, movedToProject) {
		t.Errorf("redundant area clear = %#v, want no-op %#v", redundantAreaClear, movedToProject)
	}

	movedToInbox, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{
			Project: task.ProjectChange{Clear: true},
			Area:    task.AreaChange{Clear: true},
		},
		"2026-01-07T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(project to inbox) error = %v", err)
	}
	if movedToInbox.ProjectID != nil || movedToInbox.AreaID != nil ||
		movedToInbox.Position != inboxAnchor.Position+1 {
		t.Errorf("project-to-inbox task = %#v, want uncontained append", movedToInbox)
	}

	restatedInbox, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Clear: true}},
		"2026-01-08T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate inbox) error = %v", err)
	}
	if !reflect.DeepEqual(restatedInbox, movedToInbox) {
		t.Errorf("restated inbox = %#v, want no-op %#v", restatedInbox, movedToInbox)
	}

	missingAreaID := int64(99)
	blockedTitle := "must not persist"
	_, err = tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{
			Area:  task.AreaChange{Set: &missingAreaID},
			Title: &blockedTitle,
		},
		"2026-01-08T00:00:00.000Z",
	)
	if errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(move to missing area) error = %v, want not_found", err)
	}
	persisted, err := tasks.Find(ctx, moving.ID)
	if err != nil {
		t.Fatalf("Find(after missing-area move) error = %v", err)
	}
	if !reflect.DeepEqual(persisted, movedToInbox) {
		t.Errorf("task after missing-area move = %#v, want unchanged %#v", persisted, movedToInbox)
	}
}

func TestTaskEditArchivedAreaGuardsAllowNoOpsAndGatherMoveBlockers(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	sharedArea := addStoredArea(t, areas, area.AddFields{Title: "shared"})
	otherArea := addStoredArea(t, areas, area.AddFields{Title: "other"})
	sharedProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &sharedArea.ID, Title: "shared project"},
	)
	blockedProject := addStoredProject(
		t,
		projects,
		project.AddFields{AreaID: &otherArea.ID, Title: "blocked project"},
	)
	direct := addStoredTask(t, tasks, task.AddFields{AreaID: &sharedArea.ID, Title: "direct"})
	inherited := addStoredTask(
		t,
		tasks,
		task.AddFields{ProjectID: &sharedProject.ID, Title: "inherited"},
	)
	inbox := addStoredTask(t, tasks, task.AddFields{Title: "inbox"})
	if _, err := projects.Resolve(
		ctx,
		blockedProject.ID,
		project.ExitCancelled,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(blocked project) error = %v", err)
	}
	archiveStoredAreas(t, storage, otherArea.ID, sharedArea.ID)

	directTitle := "direct content allowed"
	directEdited, err := tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{Title: &directTitle},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content in direct archived area) error = %v", err)
	}
	directRestated, err := tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{Area: task.AreaChange{Set: &sharedArea.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate direct archived area) error = %v", err)
	}
	if !reflect.DeepEqual(directRestated, directEdited) {
		t.Errorf("restated direct task = %#v, want no-op %#v", directRestated, directEdited)
	}

	inheritedTitle := "inherited content allowed"
	inheritedEdited, err := tasks.Edit(
		ctx,
		inherited.ID,
		task.EditFields{
			Project: task.ProjectChange{Set: &sharedProject.ID},
			Title:   &inheritedTitle,
		},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content with restated inherited container) error = %v", err)
	}
	if inheritedEdited.Title != inheritedTitle || inheritedEdited.Position != inherited.Position {
		t.Errorf("inherited content edit = %#v, want content change without movement", inheritedEdited)
	}

	_, err = tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{Project: task.ProjectChange{Set: &sharedProject.ID}},
		"2026-01-05T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict {
		t.Errorf("Edit(between containers under same archived area) error = %v, want conflict", err)
	} else {
		assertArchivedAreaIDs(t, err, []int64{sharedArea.ID})
	}

	_, err = tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{Area: task.AreaChange{Clear: true}},
		"2026-01-05T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict {
		t.Errorf("Edit(move out of archived area) error = %v, want conflict", err)
	} else {
		assertArchivedAreaIDs(t, err, []int64{sharedArea.ID})
	}

	_, err = tasks.Edit(
		ctx,
		inbox.ID,
		task.EditFields{Area: task.AreaChange{Set: &sharedArea.ID}},
		"2026-01-05T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict {
		t.Errorf("Edit(move into archived area) error = %v, want conflict", err)
	} else {
		assertArchivedAreaIDs(t, err, []int64{sharedArea.ID})
	}

	blockedTitle := "must not persist"
	_, err = tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{
			Project: task.ProjectChange{Set: &blockedProject.ID},
			Title:   &blockedTitle,
		},
		"2026-01-06T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict ||
		!strings.Contains(err.Error(), fmt.Sprintf("project %d", blockedProject.ID)) {
		t.Errorf("Edit(multi-block move) error = %v, want resolved project and archived areas conflict", err)
	} else {
		assertArchivedAreaIDs(t, err, []int64{sharedArea.ID, otherArea.ID})
	}
	persisted, err := tasks.Find(ctx, direct.ID)
	if err != nil {
		t.Fatalf("Find(after rejected moves) error = %v", err)
	}
	if !reflect.DeepEqual(persisted, directRestated) {
		t.Errorf("task after rejected moves = %#v, want unchanged %#v", persisted, directRestated)
	}

	missingAreaID := int64(999)
	_, err = tasks.Edit(
		ctx,
		direct.ID,
		task.EditFields{Area: task.AreaChange{Set: &missingAreaID}},
		"2026-01-07T00:00:00.000Z",
	)
	if errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(archived source to missing destination) error = %v, want not_found first", err)
	}
}

func TestTaskEditEnforcesResolvedProjectMembershipGuards(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	source := addStoredProject(t, projects, project.AddFields{Title: "source"})
	target := addStoredProject(t, projects, project.AddFields{Title: "target"})
	contained := addStoredTask(t, tasks, task.AddFields{ProjectID: &source.ID, Title: "contained"})
	loose := addStoredTask(t, tasks, task.AddFields{Title: "loose"})

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET done_at = ? WHERE id = ?",
		"2026-01-02T00:00:00.000Z",
		source.ID,
	); err != nil {
		t.Fatalf("resolve source project: %v", err)
	}

	title := "content remains editable"
	contentEdited, err := tasks.Edit(
		ctx,
		contained.ID,
		task.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(content in resolved project) error = %v", err)
	}
	if contentEdited.Title != title {
		t.Errorf("content edit title = %q, want %q", contentEdited.Title, title)
	}
	restated, err := tasks.Edit(
		ctx,
		contained.ID,
		task.EditFields{Project: task.ProjectChange{Set: &source.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(restate resolved project) error = %v", err)
	}
	if !reflect.DeepEqual(restated, contentEdited) {
		t.Errorf("restated resolved membership = %#v, want no-op %#v", restated, contentEdited)
	}

	for name, fields := range map[string]task.EditFields{
		"move out": {Project: task.ProjectChange{Clear: true}},
		"move between": {
			Project: task.ProjectChange{Set: &target.ID},
		},
	} {
		_, editErr := tasks.Edit(ctx, contained.ID, fields, "2026-01-05T00:00:00.000Z")
		if errorCode(editErr) != apperr.Conflict {
			t.Errorf("Edit(%s resolved source) error = %v, want conflict", name, editErr)
		}
	}

	missingAreaID := int64(99)
	if _, err := tasks.Edit(
		ctx,
		contained.ID,
		task.EditFields{Area: task.AreaChange{Set: &missingAreaID}},
		"2026-01-05T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(resolved source to missing area) error = %v, want not_found before source conflict", err)
	}

	if _, err := storage.database.ExecContext(
		ctx,
		"UPDATE projects SET cancelled_at = ? WHERE id = ?",
		"2026-01-06T00:00:00.000Z",
		target.ID,
	); err != nil {
		t.Fatalf("resolve target project: %v", err)
	}
	blockedTitle := "must not persist"
	if _, err := tasks.Edit(
		ctx,
		loose.ID,
		task.EditFields{
			Project: task.ProjectChange{Set: &target.ID},
			Title:   &blockedTitle,
		},
		"2026-01-07T00:00:00.000Z",
	); errorCode(err) != apperr.Conflict {
		t.Errorf("Edit(move into resolved target) error = %v, want conflict", err)
	}
	unchanged, err := tasks.Find(ctx, loose.ID)
	if err != nil {
		t.Fatalf("Find(task after rejected move) error = %v", err)
	}
	if unchanged.Title != loose.Title || unchanged.ProjectID != nil || unchanged.UpdatedAt != loose.UpdatedAt {
		t.Errorf("task after rejected move = %#v, want unchanged %#v", unchanged, loose)
	}

	missingProjectID := int64(99)
	if _, err := tasks.Edit(
		ctx,
		loose.ID,
		task.EditFields{Project: task.ProjectChange{Set: &missingProjectID}},
		"2026-01-08T00:00:00.000Z",
	); errorCode(err) != apperr.NotFound {
		t.Errorf("Edit(move into missing target) error = %v, want not_found", err)
	}
}

func TestTaskMoveReportsBothResolvedProjectsTogether(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)

	source := addStoredProject(t, projects, project.AddFields{Title: "source"})
	destination := addStoredProject(t, projects, project.AddFields{Title: "destination"})
	moving := addStoredTask(t, tasks, task.AddFields{ProjectID: &source.ID, Title: "moving"})
	destinationAnchor := addStoredTask(t, tasks, task.AddFields{ProjectID: &destination.ID, Title: "anchor"})
	if _, err := projects.Resolve(
		ctx,
		source.ID,
		project.ExitDone,
		"2026-01-02T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(source) error = %v", err)
	}
	if _, err := projects.Resolve(
		ctx,
		destination.ID,
		project.ExitCancelled,
		"2026-01-03T00:00:00.000Z",
	); err != nil {
		t.Fatalf("Resolve(destination) error = %v", err)
	}

	_, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &destination.ID}},
		"2026-01-04T00:00:00.000Z",
	)
	if errorCode(err) != apperr.Conflict ||
		!strings.Contains(err.Error(), fmt.Sprint(source.ID)) ||
		!strings.Contains(err.Error(), fmt.Sprint(destination.ID)) ||
		!strings.Contains(err.Error(), "reopen both projects") {
		t.Errorf("Edit(between two resolved projects) error = %v, want both IDs and reopen-both guidance", err)
	}

	for _, id := range []int64{source.ID, destination.ID} {
		if _, err := projects.Reopen(ctx, id, "2026-01-05T00:00:00.000Z"); err != nil {
			t.Fatalf("Reopen(project %d) error = %v", id, err)
		}
	}
	moved, err := tasks.Edit(
		ctx,
		moving.ID,
		task.EditFields{Project: task.ProjectChange{Set: &destination.ID}},
		"2026-01-06T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(after reopening both projects) error = %v", err)
	}
	if moved.ProjectID == nil || *moved.ProjectID != destination.ID ||
		moved.Position != destinationAnchor.Position+1 {
		t.Errorf("Edit(after reopening both projects) = %#v, want append to destination project", moved)
	}
}

func assertArchivedAreaIDs(t *testing.T, err error, want []int64) {
	t.Helper()

	var marker *area.ArchivedAreasError
	if !errors.As(err, &marker) {
		t.Errorf("error = %v, want archived-area marker", err)
		return
	}
	if !reflect.DeepEqual(marker.IDs, want) {
		t.Errorf("archived area IDs = %v, want %v", marker.IDs, want)
	}
}

func archiveStoredAreas(t *testing.T, storage *DB, ids ...int64) {
	t.Helper()

	for _, id := range ids {
		if _, err := storage.database.ExecContext(
			context.Background(),
			"UPDATE areas SET archived_at = ? WHERE id = ?",
			"2026-01-10T00:00:00.000Z",
			id,
		); err != nil {
			t.Fatalf("archive area %d fixture: %v", id, err)
		}
	}
}

func openTestStorage(t *testing.T) (context.Context, *DB) {
	t.Helper()

	ctx := context.Background()
	storage, err := Open(ctx, filepath.Join(t.TempDir(), "gsd.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return ctx, storage
}

func addStoredProject(t *testing.T, projects *Projects, fields project.AddFields) project.Project {
	t.Helper()

	created, err := projects.Add(context.Background(), fields, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(%q) error = %v", fields.Title, err)
	}
	return created
}

func addStoredTask(t *testing.T, tasks *Tasks, fields task.AddFields) task.Task {
	t.Helper()

	created, err := tasks.Add(context.Background(), fields, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(%q) error = %v", fields.Title, err)
	}

	return created
}
