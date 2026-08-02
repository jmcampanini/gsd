package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestProjectTagsRoundTripInTagIDOrderAcrossProjectOperations(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tags := NewTags(storage)

	blue := addProjectTestTag(t, tags, "Blue")
	amber := addProjectTestTag(t, tags, "amber")
	container := addStoredArea(t, areas, area.AddFields{Title: "container"})
	created, err := projects.Add(ctx, project.AddFields{
		AreaID: &container.ID,
		Title:  "tagged primitives",
		Tags:   []string{"Blue", "amber"},
	}, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(project) error = %v", err)
	}
	if created.Tags == nil || len(created.Tags) != 0 {
		t.Fatalf("Add(project) tags = %#v, want nonnil empty base insert", created.Tags)
	}

	resolved, err := projects.ResolveTags(ctx, []string{"amber", "blue"})
	if err != nil {
		t.Fatalf("ResolveTags() error = %v", err)
	}
	if err := projects.AttachTags(ctx, created.ID, append(resolved, amber)); err != nil {
		t.Fatalf("AttachTags() error = %v", err)
	}

	found, err := projects.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(tagged project) error = %v", err)
	}
	if !reflect.DeepEqual(found.Tags, []string{blue.Title, amber.Title}) {
		t.Errorf("Find(tagged project) tags = %v, want tag-ID order", found.Tags)
	}
	if found.UpdatedAt != created.UpdatedAt {
		t.Errorf("updated_at after attach = %q, want unchanged %q", found.UpdatedAt, created.UpdatedAt)
	}

	for _, options := range []project.ListOptions{
		{Status: project.ListStatusAll},
		{Status: project.ListStatusAll, AreaID: &container.ID},
	} {
		listed, listErr := projects.List(ctx, options)
		if listErr != nil {
			t.Fatalf("List(%+v) error = %v", options, listErr)
		}
		if len(listed) != 1 || !reflect.DeepEqual(listed[0].Tags, found.Tags) {
			t.Errorf("List(%+v) = %#v, want tagged project", options, listed)
		}
	}

	title := "edited"
	edited, err := projects.Edit(
		ctx,
		created.ID,
		project.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit(project) error = %v", err)
	}
	if !reflect.DeepEqual(edited.Tags, found.Tags) {
		t.Errorf("Edit(project) tags = %v, want %v", edited.Tags, found.Tags)
	}
	completed, err := projects.Resolve(ctx, created.ID, project.ExitDone, "2026-01-04T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Resolve(project) error = %v", err)
	}
	if !reflect.DeepEqual(completed.Tags, found.Tags) {
		t.Errorf("Resolve(project) tags = %v, want %v", completed.Tags, found.Tags)
	}
	reopened, err := projects.Reopen(ctx, created.ID, "2026-01-05T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Reopen(project) error = %v", err)
	}
	if !reflect.DeepEqual(reopened.Tags, found.Tags) {
		t.Errorf("Reopen(project) tags = %v, want %v", reopened.Tags, found.Tags)
	}

	if err := projects.DetachTags(ctx, created.ID, []tag.Tag{blue}); err != nil {
		t.Fatalf("DetachTags() error = %v", err)
	}
	detached, err := projects.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(detached project) error = %v", err)
	}
	if !reflect.DeepEqual(detached.Tags, []string{amber.Title}) {
		t.Errorf("tags after detach = %v, want [%s]", detached.Tags, amber.Title)
	}
	if detached.UpdatedAt != reopened.UpdatedAt {
		t.Errorf("updated_at after detach = %q, want unchanged %q", detached.UpdatedAt, reopened.UpdatedAt)
	}

	deleted, err := projects.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete(tagged project) error = %v", err)
	}
	if !reflect.DeepEqual(deleted.Tags, detached.Tags) {
		t.Errorf("Delete(tagged project) tags = %v, want pre-delete %v", deleted.Tags, detached.Tags)
	}
}

func TestProjectTagPrimitivesShareTransactionsAndAllowResolvedArchivedContainers(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	areas := NewAreas(storage)
	projects := NewProjects(storage)
	tags := NewTags(storage)
	marker := addProjectTestTag(t, tags, "marker")
	container := addStoredArea(t, areas, area.AddFields{Title: "container"})
	persisted := addStoredProject(t, projects, project.AddFields{
		AreaID: &container.ID,
		Title:  "persisted",
	})

	rollback := errors.New("force rollback")
	var transient project.Project
	err := projects.WithinTransaction(ctx, func(transaction project.Store) error {
		var operationErr error
		transient, operationErr = transaction.Add(ctx, project.AddFields{
			AreaID: &container.ID,
			Title:  "transient",
			Tags:   []string{marker.Title},
		}, "2026-01-02T00:00:00.000Z")
		if operationErr != nil {
			return operationErr
		}
		if transient.Tags == nil || len(transient.Tags) != 0 {
			t.Fatalf("transaction Add() tags = %#v, want nonnil empty before coordination", transient.Tags)
		}
		resolved, operationErr := transaction.ResolveTags(ctx, []string{marker.Title})
		if operationErr != nil {
			return operationErr
		}
		if operationErr = transaction.AttachTags(ctx, transient.ID, resolved); operationErr != nil {
			return operationErr
		}
		if operationErr = transaction.AttachTags(ctx, persisted.ID, resolved); operationErr != nil {
			return operationErr
		}
		hydrated, operationErr := transaction.Find(ctx, transient.ID)
		if operationErr != nil {
			return operationErr
		}
		if !reflect.DeepEqual(hydrated.Tags, []string{marker.Title}) {
			t.Fatalf("transaction Find() tags = %v, want attached marker", hydrated.Tags)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTransaction() error = %v, want forced rollback", err)
	}
	if _, err := projects.Find(ctx, transient.ID); errorCode(err) != apperr.NotFound {
		t.Errorf("Find(rolled-back tagged project) error = %v, want not_found", err)
	}
	rollbackTarget, err := projects.Find(ctx, persisted.ID)
	if err != nil {
		t.Fatalf("Find(existing project after rollback) error = %v", err)
	}
	if rollbackTarget.Tags == nil || len(rollbackTarget.Tags) != 0 {
		t.Errorf("existing project tags after rollback = %#v, want nonnil empty", rollbackTarget.Tags)
	}

	resolved, err := projects.Resolve(
		ctx,
		persisted.ID,
		project.ExitDone,
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Resolve(persisted project) error = %v", err)
	}
	if _, err := areas.Archive(ctx, container.ID, "2026-01-04T00:00:00.000Z"); err != nil {
		t.Fatalf("Archive(container) error = %v", err)
	}
	if err := projects.AttachTags(ctx, persisted.ID, []tag.Tag{marker}); err != nil {
		t.Fatalf("AttachTags(resolved archived project) error = %v", err)
	}
	tagged, err := projects.Find(ctx, persisted.ID)
	if err != nil {
		t.Fatalf("Find(resolved archived project) error = %v", err)
	}
	if !reflect.DeepEqual(tagged.Tags, []string{marker.Title}) || tagged.UpdatedAt != resolved.UpdatedAt {
		t.Errorf("resolved archived project after attach = %#v, want marker and unchanged timestamp", tagged)
	}
	if err := projects.DetachTags(ctx, persisted.ID, []tag.Tag{marker}); err != nil {
		t.Fatalf("DetachTags(resolved archived project) error = %v", err)
	}
	detached, err := projects.Find(ctx, persisted.ID)
	if err != nil {
		t.Fatalf("Find(after detach) error = %v", err)
	}
	if detached.Tags == nil || len(detached.Tags) != 0 || detached.UpdatedAt != resolved.UpdatedAt {
		t.Errorf("resolved archived project after detach = %#v, want nonnil empty tags and unchanged timestamp", detached)
	}
}

func TestProjectTaskBulkMutationsReturnTaskTagsAndDeletionSnapshotsThem(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	tags := NewTags(storage)

	firstTag := addProjectTestTag(t, tags, "first")
	secondTag := addProjectTestTag(t, tags, "second")
	container := addStoredProject(t, projects, project.AddFields{Title: "container"})
	first := addStoredTask(t, tasks, task.AddFields{ProjectID: &container.ID, Title: "first"})
	second := addStoredTask(t, tasks, task.AddFields{ProjectID: &container.ID, Title: "second"})
	if _, err := storage.database.ExecContext(ctx, `
INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?), (?, ?), (?, ?)
`, first.ID, secondTag.ID, first.ID, firstTag.ID, second.ID, secondTag.ID); err != nil {
		t.Fatalf("attach task tag fixtures: %v", err)
	}

	cancelled, err := projects.CancelOpenTasks(ctx, container.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("CancelOpenTasks() error = %v", err)
	}
	if len(cancelled) != 2 ||
		!reflect.DeepEqual(cancelled[0].Tags, []string{firstTag.Title, secondTag.Title}) ||
		!reflect.DeepEqual(cancelled[1].Tags, []string{secondTag.Title}) {
		t.Fatalf("CancelOpenTasks() = %#v, want ordered task tags", cancelled)
	}

	deleted, err := projects.DeleteTasks(ctx, container.ID)
	if err != nil {
		t.Fatalf("DeleteTasks() error = %v", err)
	}
	if len(deleted) != 2 ||
		!reflect.DeepEqual(deleted[0].Tags, cancelled[0].Tags) ||
		!reflect.DeepEqual(deleted[1].Tags, cancelled[1].Tags) {
		t.Errorf("DeleteTasks() = %#v, want pre-CASCADE task tags %#v", deleted, cancelled)
	}
}

func addProjectTestTag(t *testing.T, tags *Tags, title string) tag.Tag {
	t.Helper()

	created, err := tags.Add(t.Context(), title, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(tag %q) error = %v", title, err)
	}
	return created
}
