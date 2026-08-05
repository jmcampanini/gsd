package store

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

const (
	reorderAt       = "2026-02-01T00:00:00.000Z"
	secondReorderAt = "2026-02-02T00:00:00.000Z"
)

func TestReorderSupportsEveryPlacementAndPreservesContainerScopes(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		tasks := NewTasks(storage)
		projects := NewProjects(storage)
		areas := NewAreas(storage)
		containerProject := addStoredProject(t, projects, project.AddFields{Title: "project"})
		containerArea := addStoredArea(t, areas, area.AddFields{Title: "area"})
		values := []task.Task{
			addStoredTask(t, tasks, task.AddFields{Title: "one"}),
			addStoredTask(t, tasks, task.AddFields{Title: "two"}),
			addStoredTask(t, tasks, task.AddFields{Title: "three"}),
			addStoredTask(t, tasks, task.AddFields{Title: "four"}),
		}
		projectValues := []task.Task{
			addStoredTask(t, tasks, task.AddFields{ProjectID: &containerProject.ID, Title: "project one"}),
			addStoredTask(t, tasks, task.AddFields{ProjectID: &containerProject.ID, Title: "project two"}),
		}
		areaValues := []task.Task{
			addStoredTask(t, tasks, task.AddFields{AreaID: &containerArea.ID, Title: "area one"}),
			addStoredTask(t, tasks, task.AddFields{AreaID: &containerArea.ID, Title: "area two"}),
		}

		steps := []struct {
			placement domain.Placement
			want      []int64
		}{
			{domain.Placement{Anchor: domain.PlacementFirst}, []int64{values[3].ID, values[0].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementLast}, []int64{values[0].ID, values[1].ID, values[2].ID, values[3].ID}},
			{domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: values[0].ID}, []int64{values[0].ID, values[3].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[2].ID}, []int64{values[0].ID, values[1].ID, values[3].ID, values[2].ID}},
		}
		for _, step := range steps {
			if _, err := tasks.Reorder(ctx, values[3].ID, step.placement, reorderAt); err != nil {
				t.Fatalf("Reorder(%s) error = %v", step.placement.Anchor, err)
			}
			assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS NULL", nil, step.want)
		}

		if _, err := tasks.Reorder(ctx, projectValues[1].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt); err != nil {
			t.Fatalf("Reorder(project task) error = %v", err)
		}
		assertStoredOrder(t, storage, "tasks", "project_id IS ? AND area_id IS NULL", []any{containerProject.ID}, []int64{projectValues[1].ID, projectValues[0].ID})
		assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS ?", []any{containerArea.ID}, []int64{areaValues[0].ID, areaValues[1].ID})
		if _, err := tasks.Reorder(ctx, areaValues[1].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt); err != nil {
			t.Fatalf("Reorder(area task) error = %v", err)
		}
		assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS ?", []any{containerArea.ID}, []int64{areaValues[1].ID, areaValues[0].ID})
		assertStoredOrder(t, storage, "tasks", "project_id IS ? AND area_id IS NULL", []any{containerProject.ID}, []int64{projectValues[1].ID, projectValues[0].ID})
	})

	t.Run("projects", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		projects := NewProjects(storage)
		areas := NewAreas(storage)
		container := addStoredArea(t, areas, area.AddFields{Title: "container"})
		values := []project.Project{
			addStoredProject(t, projects, project.AddFields{Title: "one"}),
			addStoredProject(t, projects, project.AddFields{Title: "two"}),
			addStoredProject(t, projects, project.AddFields{Title: "three"}),
			addStoredProject(t, projects, project.AddFields{Title: "four"}),
		}
		contained := []project.Project{
			addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "area one"}),
			addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "area two"}),
		}

		steps := []struct {
			placement domain.Placement
			want      []int64
		}{
			{domain.Placement{Anchor: domain.PlacementFirst}, []int64{values[3].ID, values[0].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementLast}, []int64{values[0].ID, values[1].ID, values[2].ID, values[3].ID}},
			{domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: values[0].ID}, []int64{values[0].ID, values[3].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[2].ID}, []int64{values[0].ID, values[1].ID, values[3].ID, values[2].ID}},
		}
		for _, step := range steps {
			if _, err := projects.Reorder(ctx, values[3].ID, step.placement, reorderAt); err != nil {
				t.Fatalf("Reorder(%s) error = %v", step.placement.Anchor, err)
			}
			assertStoredOrder(t, storage, "projects", "area_id IS NULL", nil, step.want)
		}

		if _, err := projects.Reorder(ctx, contained[1].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt); err != nil {
			t.Fatalf("Reorder(area project) error = %v", err)
		}
		assertStoredOrder(t, storage, "projects", "area_id IS ?", []any{container.ID}, []int64{contained[1].ID, contained[0].ID})
		assertStoredOrder(t, storage, "projects", "area_id IS NULL", nil, []int64{values[0].ID, values[1].ID, values[3].ID, values[2].ID})
	})

	t.Run("areas", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		areas := NewAreas(storage)
		values := []area.Area{
			addStoredArea(t, areas, area.AddFields{Title: "one"}),
			addStoredArea(t, areas, area.AddFields{Title: "two"}),
			addStoredArea(t, areas, area.AddFields{Title: "three"}),
			addStoredArea(t, areas, area.AddFields{Title: "four"}),
		}
		steps := []struct {
			placement domain.Placement
			want      []int64
		}{
			{domain.Placement{Anchor: domain.PlacementFirst}, []int64{values[3].ID, values[0].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementLast}, []int64{values[0].ID, values[1].ID, values[2].ID, values[3].ID}},
			{domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: values[0].ID}, []int64{values[0].ID, values[3].ID, values[1].ID, values[2].ID}},
			{domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[2].ID}, []int64{values[0].ID, values[1].ID, values[3].ID, values[2].ID}},
		}
		for _, step := range steps {
			if _, err := areas.Reorder(ctx, values[3].ID, step.placement, reorderAt); err != nil {
				t.Fatalf("Reorder(%s) error = %v", step.placement.Anchor, err)
			}
			assertStoredOrder(t, storage, "areas", "1", nil, step.want)
		}
	})
}

func TestReorderRepairsPositionsUsesStatusBlindReferencesAndReturnsTags(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		tasks := NewTasks(storage)
		values := []task.Task{
			addStoredTask(t, tasks, task.AddFields{Title: "one"}),
			addStoredTask(t, tasks, task.AddFields{Title: "done reference"}),
			addStoredTask(t, tasks, task.AddFields{Title: "tagged moved"}),
		}
		if _, err := tasks.Done(ctx, values[1].ID, "2026-01-10T00:00:00.000Z"); err != nil {
			t.Fatalf("Done(reference) error = %v", err)
		}
		attachReorderTagToTask(t, storage, tasks, values[2].ID)
		setReorderFixturePositions(t, storage, "tasks", values)
		before := storedUpdatedAt(t, storage, "tasks", []int64{values[0].ID, values[1].ID, values[2].ID})

		moved, err := tasks.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[1].ID}, reorderAt)
		if err != nil {
			t.Fatalf("Reorder(before done task) error = %v", err)
		}
		if !reflect.DeepEqual([]string(moved.Tags), []string{"reorder"}) {
			t.Errorf("Reorder() tags = %v, want [reorder]", moved.Tags)
		}
		assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS NULL", nil, []int64{values[0].ID, values[2].ID, values[1].ID})
		assertMovedOnlyTimestamp(t, storage, "tasks", before, values[2].ID, reorderAt)

		before = storedUpdatedAt(t, storage, "tasks", []int64{values[0].ID, values[1].ID, values[2].ID})
		if _, err := tasks.Reorder(ctx, values[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, secondReorderAt); err != nil {
			t.Fatalf("Reorder(no-op first) error = %v", err)
		}
		assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS NULL", nil, []int64{values[0].ID, values[2].ID, values[1].ID})
		assertMovedOnlyTimestamp(t, storage, "tasks", before, values[0].ID, secondReorderAt)
	})

	t.Run("projects", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		projects := NewProjects(storage)
		values := []project.Project{
			addStoredProject(t, projects, project.AddFields{Title: "one"}),
			addStoredProject(t, projects, project.AddFields{Title: "done reference"}),
			addStoredProject(t, projects, project.AddFields{Title: "tagged moved"}),
		}
		if _, err := projects.Resolve(ctx, values[1].ID, project.ExitDone, "2026-01-10T00:00:00.000Z"); err != nil {
			t.Fatalf("Resolve(reference) error = %v", err)
		}
		attachReorderTagToProject(t, storage, projects, values[2].ID)
		setReorderFixturePositions(t, storage, "projects", values)
		before := storedUpdatedAt(t, storage, "projects", []int64{values[0].ID, values[1].ID, values[2].ID})

		moved, err := projects.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[1].ID}, reorderAt)
		if err != nil {
			t.Fatalf("Reorder(before done project) error = %v", err)
		}
		if !reflect.DeepEqual([]string(moved.Tags), []string{"reorder"}) {
			t.Errorf("Reorder() tags = %v, want [reorder]", moved.Tags)
		}
		assertStoredOrder(t, storage, "projects", "area_id IS NULL", nil, []int64{values[0].ID, values[2].ID, values[1].ID})
		assertMovedOnlyTimestamp(t, storage, "projects", before, values[2].ID, reorderAt)

		before = storedUpdatedAt(t, storage, "projects", []int64{values[0].ID, values[1].ID, values[2].ID})
		if _, err := projects.Reorder(ctx, values[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, secondReorderAt); err != nil {
			t.Fatalf("Reorder(no-op first) error = %v", err)
		}
		assertMovedOnlyTimestamp(t, storage, "projects", before, values[0].ID, secondReorderAt)
	})

	t.Run("areas", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		areas := NewAreas(storage)
		values := []area.Area{
			addStoredArea(t, areas, area.AddFields{Title: "one"}),
			addStoredArea(t, areas, area.AddFields{Title: "archived reference"}),
			addStoredArea(t, areas, area.AddFields{Title: "tagged moved"}),
		}
		if _, err := areas.Archive(ctx, values[1].ID, "2026-01-10T00:00:00.000Z"); err != nil {
			t.Fatalf("Archive(reference) error = %v", err)
		}
		attachReorderTagToArea(t, storage, areas, values[2].ID)
		setReorderFixturePositions(t, storage, "areas", values)
		before := storedUpdatedAt(t, storage, "areas", []int64{values[0].ID, values[1].ID, values[2].ID})

		moved, err := areas.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: values[1].ID}, reorderAt)
		if err != nil {
			t.Fatalf("Reorder(before archived area) error = %v", err)
		}
		if !reflect.DeepEqual([]string(moved.Tags), []string{"reorder"}) {
			t.Errorf("Reorder() tags = %v, want [reorder]", moved.Tags)
		}
		assertStoredOrder(t, storage, "areas", "1", nil, []int64{values[0].ID, values[2].ID, values[1].ID})
		assertMovedOnlyTimestamp(t, storage, "areas", before, values[2].ID, reorderAt)

		before = storedUpdatedAt(t, storage, "areas", []int64{values[0].ID, values[1].ID, values[2].ID})
		if _, err := areas.Reorder(ctx, values[0].ID, domain.Placement{Anchor: domain.PlacementFirst}, secondReorderAt); err != nil {
			t.Fatalf("Reorder(no-op first) error = %v", err)
		}
		assertMovedOnlyTimestamp(t, storage, "areas", before, values[0].ID, secondReorderAt)
	})
}

func TestTaskReorderSupportsRelativeAndSingleElementNoOps(t *testing.T) {
	ctx, storage := openTestStorage(t)
	tasks := NewTasks(storage)
	projects := NewProjects(storage)
	first := addStoredTask(t, tasks, task.AddFields{Title: "first"})
	second := addStoredTask(t, tasks, task.AddFields{Title: "second"})
	before := storedUpdatedAt(t, storage, "tasks", []int64{first.ID, second.ID})

	moved, err := tasks.Reorder(ctx, second.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: first.ID}, reorderAt)
	if err != nil {
		t.Fatalf("Reorder(current predecessor) error = %v", err)
	}
	if moved.ID != second.ID || moved.Position != 1 || moved.UpdatedAt != reorderAt {
		t.Errorf("Reorder(current predecessor) = %#v, want second task at position 1 with new timestamp", moved)
	}
	assertStoredOrder(t, storage, "tasks", "project_id IS NULL AND area_id IS NULL", nil, []int64{first.ID, second.ID})
	assertMovedOnlyTimestamp(t, storage, "tasks", before, second.ID, reorderAt)

	container := addStoredProject(t, projects, project.AddFields{Title: "container"})
	single := addStoredTask(t, tasks, task.AddFields{ProjectID: &container.ID, Title: "single"})
	before = storedUpdatedAt(t, storage, "tasks", []int64{single.ID})
	moved, err = tasks.Reorder(ctx, single.ID, domain.Placement{Anchor: domain.PlacementLast}, secondReorderAt)
	if err != nil {
		t.Fatalf("Reorder(single element) error = %v", err)
	}
	if moved.ID != single.ID || moved.Position != 0 || moved.UpdatedAt != secondReorderAt {
		t.Errorf("Reorder(single element) = %#v, want sole task at position 0 with new timestamp", moved)
	}
	assertStoredOrder(t, storage, "tasks", "project_id IS ? AND area_id IS NULL", []any{container.ID}, []int64{single.ID})
	assertMovedOnlyTimestamp(t, storage, "tasks", before, single.ID, secondReorderAt)
}

func TestReorderCaseUpdateSupportsContainersBeyondSQLiteBindLimit(t *testing.T) {
	ctx, storage := openTestStorage(t)
	const rowCount = 10_922
	ordered := make([]int64, rowCount)
	for index := range ordered {
		ordered[index] = int64(index + 1)
	}
	clause, arguments := reorderCaseUpdate(ordered, rowCount, reorderAt)
	if _, err := storage.database.ExecContext(ctx, "UPDATE tasks SET "+clause, arguments...); err != nil {
		t.Fatalf("execute reorder update for %d rows: %v", rowCount, err)
	}
}

func TestReorderErrorsFollowExistenceSelfAndContainerPrecedence(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		tasks := NewTasks(storage)
		projects := NewProjects(storage)
		container := addStoredProject(t, projects, project.AddFields{Title: "container"})
		inbox := addStoredTask(t, tasks, task.AddFields{Title: "inbox"})
		contained := addStoredTask(t, tasks, task.AddFields{ProjectID: &container.ID, Title: "contained"})
		checks := []struct {
			id            int64
			placement     domain.Placement
			want          apperr.Code
			wantSubjectID int64
		}{
			{99, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 98}, apperr.NotFound, 99},
			{inbox.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 98}, apperr.NotFound, 0},
			{inbox.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: inbox.ID}, apperr.InvalidArgument, 0},
			{inbox.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: contained.ID}, apperr.InvalidArgument, 0},
		}
		for _, check := range checks {
			_, err := tasks.Reorder(ctx, check.id, check.placement, reorderAt)
			if errorCode(err) != check.want {
				t.Errorf("Reorder(%d, %#v) error = %v, want code %s", check.id, check.placement, err, check.want)
				continue
			}
			if check.wantSubjectID != 0 && !strings.Contains(err.Error(), fmt.Sprintf("%d", check.wantSubjectID)) {
				t.Errorf("Reorder(%d, %#v) error = %v, want subject ID %d", check.id, check.placement, err, check.wantSubjectID)
			}
		}
	})

	t.Run("projects", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		projects := NewProjects(storage)
		areas := NewAreas(storage)
		container := addStoredArea(t, areas, area.AddFields{Title: "container"})
		standalone := addStoredProject(t, projects, project.AddFields{Title: "standalone"})
		contained := addStoredProject(t, projects, project.AddFields{AreaID: &container.ID, Title: "contained"})
		checks := []struct {
			id            int64
			placement     domain.Placement
			want          apperr.Code
			wantSubjectID int64
		}{
			{99, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 98}, apperr.NotFound, 99},
			{standalone.ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: 98}, apperr.NotFound, 0},
			{standalone.ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: standalone.ID}, apperr.InvalidArgument, 0},
			{standalone.ID, domain.Placement{Anchor: domain.PlacementBefore, ReferenceID: contained.ID}, apperr.InvalidArgument, 0},
		}
		for _, check := range checks {
			_, err := projects.Reorder(ctx, check.id, check.placement, reorderAt)
			if errorCode(err) != check.want {
				t.Errorf("Reorder(%d, %#v) error = %v, want code %s", check.id, check.placement, err, check.want)
				continue
			}
			if check.wantSubjectID != 0 && !strings.Contains(err.Error(), fmt.Sprintf("%d", check.wantSubjectID)) {
				t.Errorf("Reorder(%d, %#v) error = %v, want subject ID %d", check.id, check.placement, err, check.wantSubjectID)
			}
		}
	})

	t.Run("areas", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		areas := NewAreas(storage)
		stored := addStoredArea(t, areas, area.AddFields{Title: "stored"})
		checks := []struct {
			id            int64
			placement     domain.Placement
			want          apperr.Code
			wantSubjectID int64
		}{
			{99, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 98}, apperr.NotFound, 99},
			{stored.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: 98}, apperr.NotFound, 0},
			{stored.ID, domain.Placement{Anchor: domain.PlacementAfter, ReferenceID: stored.ID}, apperr.InvalidArgument, 0},
		}
		for _, check := range checks {
			_, err := areas.Reorder(ctx, check.id, check.placement, reorderAt)
			if errorCode(err) != check.want {
				t.Errorf("Reorder(%d, %#v) error = %v, want code %s", check.id, check.placement, err, check.want)
				continue
			}
			if check.wantSubjectID != 0 && !strings.Contains(err.Error(), fmt.Sprintf("%d", check.wantSubjectID)) {
				t.Errorf("Reorder(%d, %#v) error = %v, want subject ID %d", check.id, check.placement, err, check.wantSubjectID)
			}
		}
	})
}

func TestReordersRollBackWhenFinalRereadFails(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		storage.database.SetMaxOpenConns(1)
		tasks := NewTasks(storage)
		values := []task.Task{
			addStoredTask(t, tasks, task.AddFields{Title: "one"}),
			addStoredTask(t, tasks, task.AddFields{Title: "two"}),
			addStoredTask(t, tasks, task.AddFields{Title: "three"}),
		}
		ids := []int64{values[0].ID, values[1].ID, values[2].ID}
		assertFinalReorderRereadRollback(
			t,
			storage,
			"tasks",
			"project_id IS NULL AND area_id IS NULL",
			ids,
			func() error {
				_, err := tasks.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt)
				return err
			},
		)
	})

	t.Run("projects", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		storage.database.SetMaxOpenConns(1)
		projects := NewProjects(storage)
		values := []project.Project{
			addStoredProject(t, projects, project.AddFields{Title: "one"}),
			addStoredProject(t, projects, project.AddFields{Title: "two"}),
			addStoredProject(t, projects, project.AddFields{Title: "three"}),
		}
		ids := []int64{values[0].ID, values[1].ID, values[2].ID}
		assertFinalReorderRereadRollback(
			t,
			storage,
			"projects",
			"area_id IS NULL",
			ids,
			func() error {
				_, err := projects.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt)
				return err
			},
		)
	})

	t.Run("areas", func(t *testing.T) {
		ctx, storage := openTestStorage(t)
		storage.database.SetMaxOpenConns(1)
		areas := NewAreas(storage)
		values := []area.Area{
			addStoredArea(t, areas, area.AddFields{Title: "one"}),
			addStoredArea(t, areas, area.AddFields{Title: "two"}),
			addStoredArea(t, areas, area.AddFields{Title: "three"}),
		}
		ids := []int64{values[0].ID, values[1].ID, values[2].ID}
		assertFinalReorderRereadRollback(
			t,
			storage,
			"areas",
			"1",
			ids,
			func() error {
				_, err := areas.Reorder(ctx, values[2].ID, domain.Placement{Anchor: domain.PlacementFirst}, reorderAt)
				return err
			},
		)
	})
}

func assertFinalReorderRereadRollback(
	t *testing.T,
	storage *DB,
	table string,
	condition string,
	ids []int64,
	reorder func() error,
) {
	t.Helper()
	before := storedUpdatedAt(t, storage, table, ids)
	if _, err := storage.database.ExecContext(context.Background(), fmt.Sprintf(`
CREATE TEMP TRIGGER fail_reorder_reread
AFTER UPDATE OF position ON %s
WHEN NEW.id = %d
BEGIN
    DELETE FROM %s WHERE id = NEW.id;
END`, table, ids[len(ids)-1], table)); err != nil {
		t.Fatalf("create final-reread failure trigger: %v", err)
	}

	if err := reorder(); errorCode(err) != apperr.NotFound {
		t.Fatalf("Reorder() error = %v, want final-reread not_found", err)
	}
	assertStoredOrder(t, storage, table, condition, nil, ids)
	if got := storedUpdatedAt(t, storage, table, ids); !reflect.DeepEqual(got, before) {
		t.Errorf("updated_at after rollback = %v, want %v", got, before)
	}
}

func assertStoredOrder(t *testing.T, storage *DB, table, condition string, arguments []any, want []int64) {
	t.Helper()
	rows, err := storage.database.QueryContext(
		context.Background(),
		"SELECT id, position FROM "+table+" WHERE "+condition+" ORDER BY position, id",
		arguments...,
	)
	if err != nil {
		t.Fatalf("query %s order: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	got := make([]int64, 0, len(want))
	for rows.Next() {
		var id, position int64
		if err := rows.Scan(&id, &position); err != nil {
			t.Fatalf("scan %s order: %v", table, err)
		}
		if position != int64(len(got)) {
			t.Errorf("%s row %d position = %d, want %d", table, id, position, len(got))
		}
		got = append(got, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s order: %v", table, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s order = %v, want %v", table, got, want)
	}
}

func setReorderFixturePositions[T interface {
	task.Task | project.Project | area.Area
}](
	t *testing.T,
	storage *DB,
	table string,
	values []T,
) {
	t.Helper()
	for index, value := range values {
		var id int64
		switch current := any(value).(type) {
		case task.Task:
			id = current.ID
		case project.Project:
			id = current.ID
		case area.Area:
			id = current.ID
		}
		if _, err := storage.database.ExecContext(
			context.Background(),
			"UPDATE "+table+" SET position = ? WHERE id = ?",
			4+index*5,
			id,
		); err != nil {
			t.Fatalf("set %s fixture position: %v", table, err)
		}
	}
}

func storedUpdatedAt(t *testing.T, storage *DB, table string, ids []int64) map[int64]string {
	t.Helper()
	values := make(map[int64]string, len(ids))
	for _, id := range ids {
		var updatedAt string
		if err := storage.database.QueryRowContext(
			context.Background(),
			"SELECT updated_at FROM "+table+" WHERE id = ?",
			id,
		).Scan(&updatedAt); err != nil {
			t.Fatalf("read %s %d updated_at: %v", table, id, err)
		}
		values[id] = updatedAt
	}
	return values
}

func assertMovedOnlyTimestamp(
	t *testing.T,
	storage *DB,
	table string,
	before map[int64]string,
	movedID int64,
	want string,
) {
	t.Helper()
	after := storedUpdatedAt(t, storage, table, mapKeys(before))
	for id, previous := range before {
		expected := previous
		if id == movedID {
			expected = want
		}
		if after[id] != expected {
			t.Errorf("%s %d updated_at = %q, want %q", table, id, after[id], expected)
		}
	}
}

func mapKeys(values map[int64]string) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func addReorderTag(t *testing.T, storage *DB) int64 {
	t.Helper()
	created, err := NewTags(storage).Add(context.Background(), "reorder", "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(reorder tag) error = %v", err)
	}
	return created.ID
}

func attachReorderTagToTask(t *testing.T, storage *DB, tasks *Tasks, id int64) {
	t.Helper()
	tagID := addReorderTag(t, storage)
	resolved, err := NewTags(storage).Find(context.Background(), "reorder")
	if err != nil || resolved.ID != tagID {
		t.Fatalf("Find(reorder tag) = %#v, %v", resolved, err)
	}
	if err := tasks.AttachTags(context.Background(), id, []tag.Tag{resolved}); err != nil {
		t.Fatalf("AttachTags(task) error = %v", err)
	}
}

func attachReorderTagToProject(t *testing.T, storage *DB, projects *Projects, id int64) {
	t.Helper()
	addReorderTag(t, storage)
	resolved, err := NewTags(storage).Find(context.Background(), "reorder")
	if err != nil {
		t.Fatalf("Find(reorder tag) error = %v", err)
	}
	if err := projects.AttachTags(context.Background(), id, []tag.Tag{resolved}); err != nil {
		t.Fatalf("AttachTags(project) error = %v", err)
	}
}

func attachReorderTagToArea(t *testing.T, storage *DB, areas *Areas, id int64) {
	t.Helper()
	addReorderTag(t, storage)
	resolved, err := NewTags(storage).Find(context.Background(), "reorder")
	if err != nil {
		t.Fatalf("Find(reorder tag) error = %v", err)
	}
	if err := areas.AttachTags(context.Background(), id, []tag.Tag{resolved}); err != nil {
		t.Fatalf("AttachTags(area) error = %v", err)
	}
}
