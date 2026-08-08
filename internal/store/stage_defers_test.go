package store

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestAvailableStageGateReactsToForwardAndBackwardProjectMovementIndependentlyOfDateGate(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	backlog := addStoredStage(t, boards, pipeline.ID, "Backlog", "2026-01-01T00:00:00.000Z")
	active := addStoredStage(t, boards, pipeline.ID, "ActiveSecret", "2026-01-01T00:00:00.000Z")
	otherBoard := addStoredBoard(t, boards, board.AddFields{Title: "other"}, "2026-01-01T00:00:00.000Z")
	foreign := addStoredStage(t, boards, otherBoard.ID, "Foreign", "2026-01-01T00:00:00.000Z")
	contained := addStoredProject(t, projects, project.CreateFields{StageID: &backlog.ID, Title: "contained"})
	future := "9999-12-31"
	past := "0000-01-01"

	plain := addStoredTask(t, tasks, task.AddFields{ProjectID: &contained.ID, Title: "plain"})
	stageOnly := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &contained.ID, Title: "stage", DeferStageID: &active.ID, Promotes: true,
	})
	dateOnly := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &contained.ID, Title: "date", DeferUntil: &future,
	})
	both := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &contained.ID, Title: "both", DeferUntil: &future, DeferStageID: &active.ID,
	})
	foreignDeferred := addStoredTask(t, tasks, task.AddFields{
		ProjectID: &contained.ID, Title: "foreign", DeferStageID: &foreign.ID,
	})

	if got := availableTaskIDs(t, tasks); !reflect.DeepEqual(got, []int64{plain.ID}) {
		t.Errorf("available before stage/date gates = %v, want only plain task", got)
	}
	found, err := tasks.Find(ctx, stageOnly.ID)
	if err != nil {
		t.Fatalf("Find(stage-deferred) error = %v", err)
	}
	if found.DeferStageID == nil || *found.DeferStageID != active.ID ||
		found.DeferStageTitle == nil || *found.DeferStageTitle != active.Title || !found.Promotes || found.Tags == nil {
		t.Errorf("stage-deferred task = %#v, want new fields, hidden title, and non-nil tags", found)
	}
	encoded, err := json.Marshal(found)
	if err != nil {
		t.Fatalf("marshal task: %v", err)
	}
	if strings.Contains(string(encoded), active.Title) || strings.Contains(string(encoded), "defer_stage_title") {
		t.Errorf("task JSON = %s, want hidden stage title omitted", encoded)
	}

	if _, err := projects.MoveStage(ctx, contained.ID, active.ID, domain.Placement{}, "2026-01-02T00:00:00.000Z"); err != nil {
		t.Fatalf("MoveStage(forward) error = %v", err)
	}
	if got := availableTaskIDs(t, tasks); !reflect.DeepEqual(got, []int64{plain.ID, stageOnly.ID}) {
		t.Errorf("available after stage gate = %v, want plain and stage-only tasks", got)
	}
	if _, err := storage.database.ExecContext(ctx, "UPDATE tasks SET defer_until = ? WHERE id IN (?, ?)", past, dateOnly.ID, both.ID); err != nil {
		t.Fatalf("pass date gates: %v", err)
	}
	if got := availableTaskIDs(t, tasks); !reflect.DeepEqual(got, []int64{plain.ID, stageOnly.ID, dateOnly.ID, both.ID}) {
		t.Errorf("available after independent date and stage gates = %v, want all same-board tasks", got)
	}

	if _, err := projects.MoveStage(ctx, contained.ID, backlog.ID, domain.Placement{}, "2026-01-03T00:00:00.000Z"); err != nil {
		t.Fatalf("MoveStage(backward) error = %v", err)
	}
	if got := availableTaskIDs(t, tasks); !reflect.DeepEqual(got, []int64{plain.ID, dateOnly.ID}) {
		t.Errorf("available after backward move = %v, want stage-deferred tasks hidden again", got)
	}
	deferred, err := tasks.List(ctx, task.ListFilter{Status: task.ListStatusAll, Date: task.DateSelectorDeferred})
	if err != nil {
		t.Fatalf("List(deferred) error = %v", err)
	}
	if got := taskIDs(deferred); !reflect.DeepEqual(got, []int64{stageOnly.ID, both.ID, foreignDeferred.ID}) {
		t.Errorf("deferred list IDs = %v, want unreached same-board and foreign-stage tasks", got)
	}
}

func TestStageDeferClearOperationsReturnCompleteDeterministicallySortedTasks(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	stage := addStoredStage(t, boards, pipeline.ID, "later", "2026-01-01T00:00:00.000Z")
	firstProject := addStoredProject(t, projects, project.CreateFields{StageID: &stage.ID, Title: "first"})
	secondProject := addStoredProject(t, projects, project.CreateFields{StageID: &stage.ID, Title: "second"})
	first := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "first", DeferStageID: &stage.ID, Promotes: true})
	second := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "second", DeferStageID: &stage.ID})
	untouched := addStoredTask(t, tasks, task.AddFields{ProjectID: &firstProject.ID, Title: "untouched"})
	other := addStoredTask(t, tasks, task.AddFields{ProjectID: &secondProject.ID, Title: "other", DeferStageID: &stage.ID})
	if _, err := storage.database.ExecContext(ctx, "UPDATE tasks SET position = CASE id WHEN ? THEN 2 WHEN ? THEN 0 END WHERE id IN (?, ?)", first.ID, second.ID, first.ID, second.ID); err != nil {
		t.Fatalf("arrange task positions: %v", err)
	}
	clearedByEdit, err := tasks.Edit(ctx, second.ID, task.EditFields{
		DeferStageID: task.IDChange{Clear: true},
	}, "2026-01-02T00:00:00.000Z")
	if err != nil || clearedByEdit.DeferStageID != nil || clearedByEdit.DeferStageTitle != nil {
		t.Fatalf("Edit(clear defer stage) = %#v, %v; want cleared stage fields", clearedByEdit, err)
	}
	promotes := true
	restoredByEdit, err := tasks.Edit(ctx, second.ID, task.EditFields{
		DeferStageID: task.IDChange{Set: &stage.ID},
		Promotes:     &promotes,
	}, "2026-01-03T00:00:00.000Z")
	if err != nil || restoredByEdit.DeferStageID == nil || *restoredByEdit.DeferStageID != stage.ID ||
		restoredByEdit.DeferStageTitle == nil || *restoredByEdit.DeferStageTitle != stage.Title || !restoredByEdit.Promotes {
		t.Fatalf("Edit(restore defer stage and promotes) = %#v, %v; want complete new fields", restoredByEdit, err)
	}

	clearedAt := "2026-01-04T00:00:00.000Z"
	cleared, err := projects.ClearTaskStageDefers(ctx, firstProject.ID, clearedAt)
	if err != nil {
		t.Fatalf("ClearTaskStageDefers(project) error = %v", err)
	}
	if got := taskIDs(cleared); !reflect.DeepEqual(got, []int64{second.ID, first.ID}) {
		t.Fatalf("project-cleared task IDs = %v, want position then ID order", got)
	}
	for _, value := range cleared {
		if value.DeferStageID != nil || value.DeferStageTitle != nil || value.UpdatedAt != clearedAt || value.Tags == nil {
			t.Errorf("project-cleared task = %#v, want complete cleared row", value)
		}
	}
	persistedUntouched, err := tasks.Find(ctx, untouched.ID)
	if err != nil || !reflect.DeepEqual(persistedUntouched, untouched) {
		t.Errorf("untouched task = %#v, %v; want %#v", persistedUntouched, err, untouched)
	}
	persistedOther, err := tasks.Find(ctx, other.ID)
	if err != nil || persistedOther.DeferStageID == nil || *persistedOther.DeferStageID != stage.ID {
		t.Errorf("other project task = %#v, %v; want stage defer preserved", persistedOther, err)
	}

	boardClearedAt := "2026-01-05T00:00:00.000Z"
	boardCleared, err := boards.ClearTaskStageDefers(ctx, stage.ID, boardClearedAt)
	if err != nil {
		t.Fatalf("ClearTaskStageDefers(stage) error = %v", err)
	}
	if len(boardCleared) != 1 || boardCleared[0].ID != other.ID || boardCleared[0].DeferStageID != nil ||
		boardCleared[0].UpdatedAt != boardClearedAt || boardCleared[0].Tags == nil {
		t.Errorf("stage-cleared tasks = %#v, want complete cleared other-project task", boardCleared)
	}
}

func TestTaskStageTransactionFindsNextAndMovesProjectByAppending(t *testing.T) {
	t.Parallel()

	ctx, storage := openTestStorage(t)
	boards := NewBoards(storage)
	projects := NewProjects(storage)
	tasks := NewTasks(storage)
	pipeline := addStoredBoard(t, boards, board.AddFields{Title: "pipeline"}, "2026-01-01T00:00:00.000Z")
	firstStage := addStoredStage(t, boards, pipeline.ID, "first", "2026-01-01T00:00:00.000Z")
	secondStage := addStoredStage(t, boards, pipeline.ID, "second", "2026-01-01T00:00:00.000Z")
	anchor := addStoredProject(t, projects, project.CreateFields{StageID: &secondStage.ID, Title: "anchor"})
	moving := addStoredProject(t, projects, project.CreateFields{StageID: &firstStage.ID, Title: "moving"})

	foundProject, err := tasks.FindProject(ctx, moving.ID)
	if err != nil || foundProject.ID != moving.ID {
		t.Fatalf("FindProject() = %#v, %v; want moving project", foundProject, err)
	}
	foundStage, err := tasks.FindStage(ctx, pipeline.ID, "SECOND")
	if err != nil || foundStage.ID != secondStage.ID || foundStage.Title != secondStage.Title {
		t.Fatalf("FindStage() = %#v, %v; want second stage", foundStage, err)
	}
	byID, err := tasks.FindStageByID(ctx, firstStage.ID)
	if err != nil || byID.ID != firstStage.ID {
		t.Fatalf("FindStageByID() = %#v, %v; want first stage", byID, err)
	}
	if exists, err := tasks.StageExists(ctx, "SECOND"); err != nil || !exists {
		t.Fatalf("StageExists(second) = %t, %v; want true", exists, err)
	}
	next, err := tasks.FindNextStage(ctx, pipeline.ID, firstStage.Position)
	if err != nil || next == nil || next.ID != secondStage.ID {
		t.Fatalf("FindNextStage() = %#v, %v; want second stage", next, err)
	}
	if next, err := tasks.FindNextStage(ctx, pipeline.ID, secondStage.Position); err != nil || next != nil {
		t.Fatalf("FindNextStage(last) = %#v, %v; want nil", next, err)
	}

	moved, err := tasks.MoveProjectStage(ctx, moving.ID, secondStage.ID, "2026-01-02T00:00:00.000Z")
	if err != nil {
		t.Fatalf("MoveProjectStage() error = %v", err)
	}
	if moved.StageID == nil || *moved.StageID != secondStage.ID || moved.StagePosition == nil || *moved.StagePosition != 1 {
		t.Errorf("MoveProjectStage() = %#v, want append after project %d", moved, anchor.ID)
	}
}

func availableTaskIDs(t *testing.T, tasks *Tasks) []int64 {
	t.Helper()
	available, err := tasks.Available(context.Background())
	if err != nil {
		t.Fatalf("Available() error = %v", err)
	}
	ids := make([]int64, len(available))
	for index := range available {
		ids[index] = available[index].ID
		if available[index].Tags == nil {
			t.Errorf("available task %d tags = nil, want []", available[index].ID)
		}
	}
	return ids
}
