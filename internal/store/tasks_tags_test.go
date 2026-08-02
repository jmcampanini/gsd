package store

import (
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestTaskTagsRoundTripAcrossDirectAndViewOperations(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tasks := NewTasks(storage)
	tags := NewTags(storage)

	zulu := addTaskTestTag(t, tags, "Zulu")
	alpha := addTaskTestTag(t, tags, "alpha")
	middle := addTaskTestTag(t, tags, "Middle")
	created := addStoredTask(t, tasks, task.AddFields{Title: "direct"})
	if created.Tags == nil || len(created.Tags) != 0 {
		t.Fatalf("Add() tags = %#v, want nonnil empty", created.Tags)
	}
	if err := tasks.AttachTags(ctx, created.ID, []tag.Tag{middle, zulu, alpha}); err != nil {
		t.Fatalf("AttachTags(direct) error = %v", err)
	}
	wantTags := []string{zulu.Title, alpha.Title, middle.Title}

	found, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if !reflect.DeepEqual(found.Tags, wantTags) {
		t.Errorf("Find() tags = %v, want tag-ID order %v", found.Tags, wantTags)
	}

	listed, err := tasks.List(ctx, task.ListOptions{Status: task.ListStatusAll})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || !reflect.DeepEqual(listed[0].Tags, wantTags) {
		t.Fatalf("List() = %#v, want tagged task", listed)
	}

	title := "edited"
	edited, err := tasks.Edit(
		ctx,
		created.ID,
		task.EditFields{Title: &title},
		"2026-01-03T00:00:00.000Z",
	)
	if err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if !reflect.DeepEqual(edited.Tags, wantTags) {
		t.Errorf("Edit() tags = %v, want %v", edited.Tags, wantTags)
	}
	done, err := tasks.Done(ctx, created.ID, "2026-01-04T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Done() error = %v", err)
	}
	if !reflect.DeepEqual(done.Tags, wantTags) {
		t.Errorf("Done() tags = %v, want %v", done.Tags, wantTags)
	}
	reopened, err := tasks.Reopen(ctx, created.ID, "2026-01-05T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if !reflect.DeepEqual(reopened.Tags, wantTags) {
		t.Errorf("Reopen() tags = %v, want %v", reopened.Tags, wantTags)
	}
	cancelled, err := tasks.Cancel(ctx, created.ID, "2026-01-06T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if !reflect.DeepEqual(cancelled.Tags, wantTags) {
		t.Errorf("Cancel() tags = %v, want %v", cancelled.Tags, wantTags)
	}

	viewTagged := addStoredTask(t, tasks, task.AddFields{Title: "view tagged"})
	if err := tasks.AttachTags(ctx, viewTagged.ID, []tag.Tag{middle, zulu}); err != nil {
		t.Fatalf("AttachTags(view task) error = %v", err)
	}
	untagged := addStoredTask(t, tasks, task.AddFields{Title: "untagged"})
	wantViewTags := []string{zulu.Title, middle.Title}
	views := []struct {
		name string
		list func() ([]task.ViewTask, error)
	}{
		{name: "Inbox", list: func() ([]task.ViewTask, error) { return tasks.Inbox(ctx) }},
		{name: "Available", list: func() ([]task.ViewTask, error) { return tasks.Available(ctx) }},
	}
	for _, viewCase := range views {
		view, viewErr := viewCase.list()
		if viewErr != nil {
			t.Fatalf("%s() error = %v", viewCase.name, viewErr)
		}
		if len(view) != 2 || view[0].ID != viewTagged.ID || view[1].ID != untagged.ID {
			t.Fatalf("%s() = %#v, want tagged then untagged tasks", viewCase.name, view)
		}
		if !reflect.DeepEqual(view[0].Tags, wantViewTags) {
			t.Errorf("%s(tagged) tags = %v, want %v", viewCase.name, view[0].Tags, wantViewTags)
		}
		if view[1].Tags == nil || len(view[1].Tags) != 0 {
			t.Errorf("%s(untagged) tags = %#v, want nonnil empty", viewCase.name, view[1].Tags)
		}
	}

	deleted, err := tasks.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("Delete(tagged) error = %v", err)
	}
	if !reflect.DeepEqual(deleted.Tags, wantTags) {
		t.Errorf("Delete(tagged) tags = %v, want pre-CASCADE snapshot %v", deleted.Tags, wantTags)
	}
	var joins int64
	if err := storage.database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM task_tags WHERE task_id = ?",
		created.ID,
	).Scan(&joins); err != nil {
		t.Fatalf("count deleted task joins: %v", err)
	}
	if joins != 0 {
		t.Errorf("deleted task join count = %d, want 0", joins)
	}
}

func TestTaskListTagFilterIsCaseInsensitiveAndComposesWithAllPredicates(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	tags := NewTags(storage)

	focus := addTaskTestTag(t, tags, "Focus")
	_ = addTaskTestTag(t, tags, "other")
	container := addStoredProject(t, projects, project.AddFields{Title: "container"})
	dueOn := "2026-08-03"
	focusDue := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &container.ID,
		Title:     "focus due",
		DueOn:     &dueOn,
	})
	focusUndated := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &container.ID,
		Title:     "focus undated",
	})
	_ = addStoredTask(t, tasks, task.AddFields{
		ProjectID: &container.ID,
		Title:     "untagged due",
		DueOn:     &dueOn,
	})
	doneFocus := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &container.ID,
		Title:     "done focus",
		DueOn:     &dueOn,
	})
	inboxFocus := addStoredTask(t, tasks, task.AddFields{Title: "inbox focus", DueOn: &dueOn})
	for _, taskID := range []int64{focusDue.ID, focusUndated.ID, doneFocus.ID, inboxFocus.ID} {
		if err := tasks.AttachTags(ctx, taskID, []tag.Tag{focus}); err != nil {
			t.Fatalf("AttachTags(task %d) error = %v", taskID, err)
		}
	}
	if _, err := tasks.Done(ctx, doneFocus.ID, "2026-01-02T00:00:00.000Z"); err != nil {
		t.Fatalf("Done(focus task) error = %v", err)
	}

	mixedCase := "fOcUs"
	listed, err := tasks.List(ctx, task.ListOptions{Status: task.ListStatusAll, Tag: &mixedCase})
	if err != nil {
		t.Fatalf("List(case-insensitive tag) error = %v", err)
	}
	gotIDs := make([]int64, len(listed))
	for index := range listed {
		gotIDs[index] = listed[index].ID
	}
	wantIDs := []int64{focusDue.ID, inboxFocus.ID, focusUndated.ID, doneFocus.ID}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("List(case-insensitive tag) IDs = %v, want position/ID order %v", gotIDs, wantIDs)
	}

	composed, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusOpen,
		Date:      task.DateSelectorDue,
		ProjectID: &container.ID,
		Tag:       &mixedCase,
	})
	if err != nil {
		t.Fatalf("List(composed filters) error = %v", err)
	}
	if len(composed) != 1 || composed[0].ID != focusDue.ID {
		t.Errorf("List(composed filters) = %#v, want only task %d", composed, focusDue.ID)
	}

	other := "OTHER"
	empty, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusAll,
		ProjectID: &container.ID,
		Tag:       &other,
	})
	if err != nil {
		t.Fatalf("List(existing container with empty tag match) error = %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("List(existing container with empty tag match) = %#v, want nonnil empty", empty)
	}

	unknown := "missing-tag"
	if _, err := tasks.List(ctx, task.ListOptions{
		Status: task.ListStatusAll,
		Tag:    &unknown,
	}); errorCode(err) != apperr.NotFound || !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("List(unknown tag) error = %v, want not_found wrapping sql.ErrNoRows", err)
	}
	missingProjectID := int64(999)
	if _, err := tasks.List(ctx, task.ListOptions{
		Status:    task.ListStatusAll,
		ProjectID: &missingProjectID,
		Tag:       &unknown,
	}); errorCode(err) != apperr.NotFound || !strings.Contains(err.Error(), "no project 999") {
		t.Errorf("List(missing container and unknown tag) error = %v, want preserved container not_found", err)
	}
}

func TestTaskTagPrimitivesAreIdempotentAndShareTransactionRefreshAndRollback(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	tasks := NewTasks(storage)
	tags := NewTags(storage)

	alpha := addTaskTestTag(t, tags, "Alpha")
	beta := addTaskTestTag(t, tags, "beta")
	gamma := addTaskTestTag(t, tags, "Gamma")
	created := addStoredTask(t, tasks, task.AddFields{Title: "transactional tags"})

	resolved, err := tasks.ResolveTags(ctx, []string{"ALPHA"})
	if err != nil {
		t.Fatalf("ResolveTags(case-insensitive) error = %v", err)
	}
	if len(resolved) != 1 || resolved[0].ID != alpha.ID || resolved[0].Title != alpha.Title {
		t.Fatalf("ResolveTags(case-insensitive) = %#v, want stored Alpha", resolved)
	}
	for attempt := range 2 {
		if err := tasks.AttachTags(ctx, created.ID, []tag.Tag{resolved[0], resolved[0]}); err != nil {
			t.Fatalf("AttachTags(attempt %d) error = %v", attempt, err)
		}
		if err := tasks.DetachTags(ctx, created.ID, []tag.Tag{gamma, gamma}); err != nil {
			t.Fatalf("DetachTags(unattached attempt %d) error = %v", attempt, err)
		}
	}
	attached, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(after idempotent attach) error = %v", err)
	}
	if !reflect.DeepEqual(attached.Tags, []string{alpha.Title}) {
		t.Errorf("tags after idempotent attach/detach = %v, want [%s]", attached.Tags, alpha.Title)
	}
	if attached.UpdatedAt != created.UpdatedAt {
		t.Errorf("updated_at after tag mutations = %q, want unchanged %q", attached.UpdatedAt, created.UpdatedAt)
	}

	rollback := errors.New("force rollback")
	err = tasks.WithinTransaction(ctx, func(transaction task.Store) error {
		if operationErr := transaction.AttachTags(ctx, created.ID, []tag.Tag{beta}); operationErr != nil {
			return operationErr
		}
		refreshed, operationErr := transaction.Find(ctx, created.ID)
		if operationErr != nil {
			return operationErr
		}
		if !reflect.DeepEqual(refreshed.Tags, []string{alpha.Title, beta.Title}) {
			t.Fatalf("transaction Find() tags = %v, want refreshed attachment", refreshed.Tags)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("WithinTransaction(rollback) error = %v, want forced rollback", err)
	}
	rolledBack, err := tasks.Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("Find(after rollback) error = %v", err)
	}
	if !reflect.DeepEqual(rolledBack.Tags, []string{alpha.Title}) {
		t.Errorf("tags after rollback = %v, want [%s]", rolledBack.Tags, alpha.Title)
	}

	var committed task.Task
	if err := tasks.WithinTransaction(ctx, func(transaction task.Store) error {
		if operationErr := transaction.AttachTags(ctx, created.ID, []tag.Tag{beta}); operationErr != nil {
			return operationErr
		}
		var operationErr error
		committed, operationErr = transaction.Find(ctx, created.ID)
		return operationErr
	}); err != nil {
		t.Fatalf("WithinTransaction(commit) error = %v", err)
	}
	if !reflect.DeepEqual(committed.Tags, []string{alpha.Title, beta.Title}) {
		t.Errorf("committed refresh tags = %v, want Alpha then beta", committed.Tags)
	}
	if committed.UpdatedAt != created.UpdatedAt {
		t.Errorf("committed updated_at = %q, want unchanged %q", committed.UpdatedAt, created.UpdatedAt)
	}
}

func addTaskTestTag(t *testing.T, tags *Tags, title string) tag.Tag {
	t.Helper()

	created, err := tags.Add(t.Context(), title, "2026-01-01T00:00:00.000Z")
	if err != nil {
		t.Fatalf("Add(tag %q) error = %v", title, err)
	}
	return created
}
