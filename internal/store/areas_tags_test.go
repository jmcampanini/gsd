package store

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestAreaTagsRoundTripInCreationOrderAcrossAreaOperations(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	tags := NewTags(storage)
	firstTag := addAreaTestTag(t, tags, "Zulu")
	secondTag := addAreaTestTag(t, tags, "alpha")
	thirdTag := addAreaTestTag(t, tags, "Middle")

	created, err := areas.Add(
		ctx,
		area.AddFields{Title: "Tagged", Tags: []string{firstTag.Title}},
		"2026-01-02T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if created.Tags == nil || len(created.Tags) != 0 {
		t.Fatalf("Add() tags = %#v, want nonnil empty tags before service attachment", created.Tags)
	}

	resolved, err := areas.ResolveTags(ctx, []string{"ALPHA", "zulu"})
	if err != nil {
		t.Fatalf("ResolveTags() error = %v", err)
	}
	if got := tag.Titles(resolved); !reflect.DeepEqual(got, []string{secondTag.Title, firstTag.Title}) {
		t.Fatalf("ResolveTags() titles = %v, want requested order with stored spelling", got)
	}
	if err := areas.AttachTags(ctx, created.ID, resolved); err != nil {
		t.Fatalf("AttachTags() error = %v", err)
	}

	found, err := areas.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	wantInitialTags := []string{firstTag.Title, secondTag.Title}
	if !reflect.DeepEqual(found.Tags, wantInitialTags) {
		t.Errorf("Find() tags = %v, want tag creation order %v", found.Tags, wantInitialTags)
	}
	if found.UpdatedAt != created.UpdatedAt {
		t.Errorf("updated_at after attach = %q, want unchanged %q", found.UpdatedAt, created.UpdatedAt)
	}

	note := "edited"
	edited, err := areas.Edit(
		ctx,
		created.ID,
		area.EditFields{Note: &note},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if !reflect.DeepEqual(edited.Tags, wantInitialTags) {
		t.Errorf("Edit() tags = %v, want %v", edited.Tags, wantInitialTags)
	}
	listed, err := areas.List(ctx, area.ListOptions{Slice: area.ListSliceAll})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || !reflect.DeepEqual(listed[0].Tags, wantInitialTags) {
		t.Errorf("List() = %#v, want tagged area", listed)
	}

	archived, err := areas.Archive(ctx, created.ID, "2026-01-04T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if !reflect.DeepEqual(archived.Tags, wantInitialTags) {
		t.Errorf("Archive() tags = %v, want %v", archived.Tags, wantInitialTags)
	}
	if err := areas.AttachTags(ctx, created.ID, []tag.Tag{thirdTag}); err != nil {
		t.Fatalf("AttachTags(archived area) error = %v", err)
	}
	archivedFound, err := areas.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(archived area) error = %v", err)
	}
	wantArchivedTags := []string{firstTag.Title, secondTag.Title, thirdTag.Title}
	if !reflect.DeepEqual(archivedFound.Tags, wantArchivedTags) {
		t.Errorf("archived area tags = %v, want %v", archivedFound.Tags, wantArchivedTags)
	}
	if archivedFound.UpdatedAt != archived.UpdatedAt {
		t.Errorf("archived updated_at after attach = %q, want unchanged %q", archivedFound.UpdatedAt, archived.UpdatedAt)
	}

	if err := areas.DetachTags(ctx, created.ID, []tag.Tag{secondTag}); err != nil {
		t.Fatalf("DetachTags() error = %v", err)
	}
	wantDetachedTags := []string{firstTag.Title, thirdTag.Title}
	detached, err := areas.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(detached area) error = %v", err)
	}
	if !reflect.DeepEqual(detached.Tags, wantDetachedTags) {
		t.Errorf("detached area tags = %v, want %v", detached.Tags, wantDetachedTags)
	}
	if detached.UpdatedAt != archived.UpdatedAt {
		t.Errorf("archived updated_at after detach = %q, want unchanged %q", detached.UpdatedAt, archived.UpdatedAt)
	}

	unarchived, err := areas.Unarchive(ctx, created.ID, "2026-01-05T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}
	if !reflect.DeepEqual(unarchived.Tags, wantDetachedTags) {
		t.Errorf("Unarchive() tags = %v, want detached tags %v", unarchived.Tags, wantDetachedTags)
	}
}

func TestAreaTaggedAddTransactionRollsBackEntityAndAttachments(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	storedTag := addAreaTestTag(t, NewTags(storage), "rollback")
	stop := errors.New("stop tagged add")
	var createdID int64

	err := areas.WithinTransaction(ctx, func(transaction area.Store) error {
		created, addErr := transaction.Add(
			ctx,
			area.AddFields{Title: "rolled back"},
			"2026-01-02T00:00:00.000Z",
		)
		if addErr != nil {
			return addErr
		}
		createdID = created.ID
		if attachErr := transaction.AttachTags(ctx, created.ID, []tag.Tag{storedTag}); attachErr != nil {
			return attachErr
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("WithinTransaction() error = %v, want %v", err, stop)
	}
	if _, err := areas.Find(ctx, createdID); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(rolled back area) error = %v, want not_found", err)
	}
	assertAreaTagJoinCount(t, storage, 0)
}

func TestAreaDeletionReturnsMaterializedTagSnapshotsAtEveryRecursiveLevel(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	tags := NewTags(storage)
	firstTag := addAreaTestTag(t, tags, "first")
	secondTag := addAreaTestTag(t, tags, "second")
	wantTags := []string{firstTag.Title, secondTag.Title}

	empty := addStoredArea(t, areas, area.AddFields{Title: "empty tagged"})
	if err := areas.AttachTags(ctx, empty.ID, []tag.Tag{secondTag, firstTag}); err != nil {
		t.Fatalf("AttachTags(empty area) error = %v", err)
	}
	deletedEmpty, err := areas.Delete(ctx, empty.ID)
	if err != nil {
		t.Fatalf("Delete(empty tagged area) error = %v", err)
	}
	if !reflect.DeepEqual(deletedEmpty.Tags, wantTags) {
		t.Errorf("Delete(empty tagged area) tags = %v, want pre-delete snapshot %v", deletedEmpty.Tags, wantTags)
	}

	doomed := addStoredArea(t, areas, area.AddFields{Title: "recursive tagged"})
	containedProject := addStoredProject(t, projects, project.AddFields{AreaID: &doomed.ID, Title: "project"})
	projectTask := addStoredTask(t, tasks, task.AddFields{ProjectID: &containedProject.ID, Title: "project task"})
	looseTask := addStoredTask(t, tasks, task.AddFields{AreaID: &doomed.ID, Title: "loose task"})
	if err := areas.AttachTags(ctx, doomed.ID, []tag.Tag{secondTag, firstTag}); err != nil {
		t.Fatalf("AttachTags(recursive area) error = %v", err)
	}
	attachEntityTagFixtures(t, storage, "project_tags", "project_id", containedProject.ID, secondTag.ID, firstTag.ID)
	attachEntityTagFixtures(t, storage, "task_tags", "task_id", projectTask.ID, secondTag.ID, firstTag.ID)
	attachEntityTagFixtures(t, storage, "task_tags", "task_id", looseTask.ID, secondTag.ID, firstTag.ID)

	var deletedArea area.Area
	var deletedProjects []project.Project
	var deletedProjectTasks []task.Task
	var deletedLooseTasks []task.Task
	err = areas.WithinTransaction(ctx, func(transaction area.Store) error {
		var operationErr error
		deletedProjectTasks, operationErr = transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeProject)
		if operationErr != nil {
			return operationErr
		}
		deletedProjects, operationErr = transaction.DeleteProjects(ctx, doomed.ID)
		if operationErr != nil {
			return operationErr
		}
		deletedLooseTasks, operationErr = transaction.DeleteTasks(ctx, doomed.ID, area.TaskDeletionScopeLoose)
		if operationErr != nil {
			return operationErr
		}
		deletedArea, operationErr = transaction.Delete(ctx, doomed.ID)
		return operationErr
	})
	if err != nil {
		t.Fatalf("WithinTransaction(recursive delete) error = %v", err)
	}
	if !reflect.DeepEqual(deletedArea.Tags, wantTags) {
		t.Errorf("deleted area tags = %v, want pre-delete snapshot %v", deletedArea.Tags, wantTags)
	}
	if len(deletedProjects) != 1 || !reflect.DeepEqual(deletedProjects[0].Tags, wantTags) {
		t.Errorf("deleted projects = %#v, want tag snapshot %v", deletedProjects, wantTags)
	}
	if len(deletedProjectTasks) != 1 || !reflect.DeepEqual(deletedProjectTasks[0].Tags, wantTags) {
		t.Errorf("deleted project tasks = %#v, want tag snapshot %v", deletedProjectTasks, wantTags)
	}
	if len(deletedLooseTasks) != 1 || !reflect.DeepEqual(deletedLooseTasks[0].Tags, wantTags) {
		t.Errorf("deleted loose tasks = %#v, want tag snapshot %v", deletedLooseTasks, wantTags)
	}
	assertAreaTagJoinCount(t, storage, 0)
}

func addAreaTestTag(t *testing.T, tags *Tags, title string) tag.Tag {
	t.Helper()

	created, err := tags.Add(context.Background(), title, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add tag %q: %v", title, err)
	}
	return created
}

func attachEntityTagFixtures(
	t *testing.T,
	storage *DB,
	table string,
	entityColumn string,
	entityID int64,
	tagIDs ...int64,
) {
	t.Helper()

	query := fmt.Sprintf("INSERT INTO %s (%s, tag_id) VALUES (?, ?)", table, entityColumn)
	for _, tagID := range tagIDs {
		if _, err := storage.database.ExecContext(context.Background(), query, entityID, tagID); err != nil {
			t.Fatalf("attach %s fixture: %v", table, err)
		}
	}
}

func assertAreaTagJoinCount(t *testing.T, storage *DB, want int64) {
	t.Helper()

	var count int64
	if err := storage.database.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM area_tags",
	).Scan(&count); err != nil {
		t.Fatalf("count area tag joins: %v", err)
	}
	if count != want {
		t.Errorf("area tag join count = %d, want %d", count, want)
	}
}
