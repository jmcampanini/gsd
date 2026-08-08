package e2e

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestBoardWorkflowAcrossBinaryInvocations(t *testing.T) {
	databasePath := filepath.Join(workDir, "boards", "gsd.db")
	runJSON := func(args ...string) processResult {
		return runGSD(t, append(args, "--db", databasePath, "--json")...)
	}

	software := decodeJSON[board.Board](t, runJSON(
		"boards", "add", "Software",
		"--stage", "Research", "--stage", "Planning", "--stage", "Doing", "--stage", "Review",
	), "board")
	other := decodeJSON[board.Board](t, runJSON(
		"boards", "add", "Other", "--stage", "Queue", "--stage", "Shipping",
	), "board")
	listed := decodeJSON[[]board.ListedBoard](t, runJSON("boards", "list"), "boards")
	if len(listed) != 2 || listed[0].ID != software.ID || listed[1].ID != other.ID {
		t.Fatalf("boards = %#v, want Software then Other", listed)
	}
	assertStageTitles(t, "initial board list", listed[0].Stages, "Research", "Planning", "Doing", "Review")
	initialShow := decodeJSON[board.Show](t, runJSON("board", "show", "software"), "board show")
	assertShownStageTitles(t, "initial board show", initialShow, "Research", "Planning", "Doing", "Review")
	for _, stage := range initialShow.Stages {
		if stage.Projects == nil || len(stage.Projects) != 0 {
			t.Errorf("initial stage %q projects = %#v, want empty non-nil array", stage.Title, stage.Projects)
		}
	}

	editedBoard := decodeJSON[board.Board](t, runJSON(
		"board", "edit", "Software", "--note", "Delivery pipeline",
	), "edited board")
	if editedBoard.Note != "Delivery pipeline" {
		t.Errorf("edited board = %#v, want persisted note", editedBoard)
	}
	decodeJSON[board.Stage](t, runJSON(
		"stage", "rename", "Software", "Planning", "Design",
	), "renamed stage")
	decodeJSON[board.Stage](t, runJSON(
		"stage", "reorder", "Software", "Design", "--after", "Doing",
	), "reordered stage")
	reshaped := decodeJSON[board.Show](t, runJSON("board", "show", "Software"), "reshaped board")
	assertShownStageTitles(t, "reshaped board", reshaped, "Research", "Doing", "Design", "Review")
	decodeJSON[board.Stage](t, runJSON(
		"stage", "reorder", "Software", "Design", "--before", "Doing",
	), "restored stage")
	decodeJSON[board.Stage](t, runJSON(
		"stages", "add", "Software", "Later", "--last",
	), "added stage")

	planned := decodeProject(t, runJSON("projects", "add", "Planned project"))
	plannedEdition := decodeJSON[project.Edition](t, runJSON(
		"project", "edit", fmt.Sprint(planned.ID), "--board", "Software",
	), "project board edition")
	if plannedEdition.Project.StageID == nil || plannedEdition.Project.StagePosition == nil ||
		len(plannedEdition.ClearedDefers) != 0 {
		t.Fatalf("project board edition = %#v, want first-stage membership and empty clears", plannedEdition)
	}
	boarded := decodeProject(t, runJSON(
		"projects", "add", "Boarded project", "--board", "Software",
	))
	humanMember := decodeProject(t, runJSON("projects", "add", "Human member"))
	humanMembership := runGSD(t,
		"project", "edit", fmt.Sprint(humanMember.ID), "--board", "Software", "--db", databasePath,
	)
	if humanMembership.exitCode != 0 || humanMembership.stderr != "" ||
		!strings.Contains(humanMembership.stdout, "Edited:") ||
		!strings.Contains(humanMembership.stdout, "Software/Research") {
		t.Fatalf("human board membership = %#v, want stored board/stage narration", humanMembership)
	}

	humanMove := runGSD(t,
		"project", "move", fmt.Sprint(planned.ID), "Design", "--db", databasePath,
	)
	if humanMove.exitCode != 0 || humanMove.stderr != "" ||
		!strings.Contains(humanMove.stdout, "Moved:") || !strings.Contains(humanMove.stdout, "→ Design") {
		t.Fatalf("human project move = %#v, want Design narration", humanMove)
	}
	movedBack := decodeProject(t, runJSON(
		"project", "move", fmt.Sprint(planned.ID), "Research", "--first",
	))
	if movedBack.StageID == nil || movedBack.StagePosition == nil || *movedBack.StagePosition != 0 {
		t.Errorf("JSON project move = %#v, want first in Research", movedBack)
	}
	sameStage := decodeProject(t, runJSON(
		"project", "move", fmt.Sprint(planned.ID), "Research", "--after", fmt.Sprint(boarded.ID),
	))
	if sameStage.StageID == nil || movedBack.StageID == nil || *sameStage.StageID != *movedBack.StageID ||
		sameStage.StagePosition == nil || *sameStage.StagePosition != 1 {
		t.Errorf("same-stage project move = %#v, want position after project %d", sameStage, boarded.ID)
	}
	movedDesign := decodeProject(t, runJSON(
		"project", "move", fmt.Sprint(planned.ID), "Design",
	))
	if movedDesign.StageID == nil {
		t.Fatalf("project in Design = %#v, want boarded project", movedDesign)
	}

	grouped := decodeJSON[board.Show](t, runJSON("board", "show", "Software"), "grouped board")
	assertShownProjectIDs(t, grouped, "Research", boarded.ID, humanMember.ID)
	assertShownProjectIDs(t, grouped, "Design", planned.ID)
	humanBoard := runGSD(t, "board", "show", "Software", "--db", databasePath)
	if humanBoard.exitCode != 0 || humanBoard.stderr != "" ||
		!strings.Contains(humanBoard.stdout, "Research → Design → Doing → Review → Later") ||
		!strings.Contains(humanBoard.stdout, planned.Title) || !strings.Contains(humanBoard.stdout, boarded.Title) {
		t.Errorf("human board show = %#v, want ordered populated board", humanBoard)
	}
	assertJSONError(t, runJSON("stage", "delete", "Software", "Design"), apperr.Conflict)
	assertJSONError(t, runJSON("board", "delete", "Software"), apperr.Conflict)

	laterDeferred := decodeTask(t, runJSON(
		"add", "Delete clears this defer", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Later",
	))
	laterDeletion := decodeJSON[board.StageDeletion](t, runJSON(
		"stage", "delete", "Software", "Later",
	), "stage deletion")
	if laterDeletion.Stage.Title != "Later" || len(laterDeletion.ClearedDefers) != 1 ||
		laterDeletion.ClearedDefers[0].ID != laterDeferred.ID ||
		laterDeletion.ClearedDefers[0].DeferStageID != nil {
		t.Errorf("stage deletion = %#v, want deleted Later and one cleared task", laterDeletion)
	}
	if persisted := decodeTask(t, runJSON("show", fmt.Sprint(laterDeferred.ID))); persisted.DeferStageID != nil {
		t.Errorf("task after stage deletion = %#v, want persisted cleared stage defer", persisted)
	}

	reparentDeferred := decodeTask(t, runJSON(
		"add", "Reparent clears this defer", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Doing",
	))
	reparented := decodeTaskEdition(t, runJSON(
		"edit", fmt.Sprint(reparentDeferred.ID), "--no-project",
	))
	if reparented.Task.ProjectID != nil || reparented.Task.DeferStageID != nil ||
		len(reparented.ClearedDefers) != 1 || reparented.ClearedDefers[0].ID != reparentDeferred.ID {
		t.Errorf("task containment edition = %#v, want task and reported defer clear", reparented)
	}
	if persisted := decodeTask(t, runJSON("show", fmt.Sprint(reparentDeferred.ID))); persisted.ProjectID != nil || persisted.DeferStageID != nil {
		t.Errorf("reparented task = %#v, want persisted containment and defer clear", persisted)
	}

	projectDeferred := decodeTask(t, runJSON(
		"add", "Board clear clears this defer", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Doing",
	))
	clearedBoard := decodeJSON[project.Edition](t, runJSON(
		"project", "edit", fmt.Sprint(planned.ID), "--no-board",
	), "project board clear")
	if clearedBoard.Project.StageID != nil || len(clearedBoard.ClearedDefers) != 1 ||
		clearedBoard.ClearedDefers[0].ID != projectDeferred.ID ||
		clearedBoard.ClearedDefers[0].DeferStageID != nil {
		t.Errorf("project board clear = %#v, want off-board project and reported defer clear", clearedBoard)
	}
	if persisted := decodeTask(t, runJSON("show", fmt.Sprint(projectDeferred.ID))); persisted.DeferStageID != nil {
		t.Errorf("task after project board clear = %#v, want persisted cleared stage defer", persisted)
	}
	decodeJSON[project.Edition](t, runJSON(
		"project", "edit", fmt.Sprint(planned.ID), "--board", "Software",
	), "project board restoration")
	decodeProject(t, runJSON("project", "move", fmt.Sprint(planned.ID), "Design"))

	stageDeferred := decodeTask(t, runJSON(
		"add", "Visible at Doing", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Doing",
	))
	futureDeferred := decodeTask(t, runJSON(
		"add", "Future date still gates", "--project", fmt.Sprint(planned.ID),
		"--defer-stage", "Doing", "--defer", "2999-12-31",
	))
	if stageDeferred.DeferStageID == nil || stageDeferred.Promotes ||
		futureDeferred.DeferStageID == nil || futureDeferred.DeferUntil == nil ||
		*futureDeferred.DeferUntil != "2999-12-31" {
		t.Fatalf("stage/date deferred tasks = %#v/%#v, want independent persisted gates", stageDeferred, futureDeferred)
	}
	availableInDesign := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available in Design")
	assertTaskVisibility(t, availableInDesign, stageDeferred.ID, false)
	assertTaskVisibility(t, availableInDesign, futureDeferred.ID, false)

	decodeProject(t, runJSON("project", "move", fmt.Sprint(planned.ID), "Doing"))
	availableInDoing := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available in Doing")
	assertTaskVisibility(t, availableInDoing, stageDeferred.ID, true)
	assertTaskVisibility(t, availableInDoing, futureDeferred.ID, false)
	decodeProject(t, runJSON("project", "move", fmt.Sprint(planned.ID), "Review"))
	availableInReview := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available in Review")
	assertTaskVisibility(t, availableInReview, stageDeferred.ID, true)
	decodeProject(t, runJSON("project", "move", fmt.Sprint(planned.ID), "Design"))
	availableBackInDesign := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available back in Design")
	assertTaskVisibility(t, availableBackInDesign, stageDeferred.ID, false)

	promoting := decodeTask(t, runJSON(
		"add", "Promote to Doing", "--project", fmt.Sprint(planned.ID), "--promotes",
	))
	if !promoting.Promotes {
		t.Fatalf("promoting task = %#v, want promotes marker", promoting)
	}
	completion := decodeJSON[task.Completion](t, runJSON(
		"done", fmt.Sprint(promoting.ID),
	), "promoting completion")
	if completion.Task.Status != "done" || completion.PromotedProject == nil ||
		completion.PromotedProject.ID != planned.ID {
		t.Fatalf("promoting completion = %#v, want task done and project returned", completion)
	}
	doingID := shownStage(t, decodeJSON[board.Show](t, runJSON(
		"board", "show", "Software",
	), "promoted board"), "Doing").ID
	if completion.PromotedProject.StageID == nil || *completion.PromotedProject.StageID != doingID {
		t.Errorf("promoted project = %#v, want Doing stage %d", completion.PromotedProject, doingID)
	}
	availableAfterPromotion := decodeJSON[[]task.ViewTask](t, runJSON("available"), "available after promotion")
	assertTaskVisibility(t, availableAfterPromotion, stageDeferred.ID, true)
	assertTaskVisibility(t, availableAfterPromotion, futureDeferred.ID, false)

	decodeTask(t, runJSON("reopen", fmt.Sprint(promoting.ID)))
	afterReopen := decodeProject(t, runJSON("project", "show", fmt.Sprint(planned.ID)))
	if afterReopen.StageID == nil || *afterReopen.StageID != doingID {
		t.Errorf("project after task reopen = %#v, want no demotion from Doing", afterReopen)
	}
	promotingHuman := decodeTask(t, runJSON(
		"add", "Promote to Review", "--project", fmt.Sprint(planned.ID), "--promotes",
	))
	humanPromotion := runGSD(t, "done", fmt.Sprint(promotingHuman.ID), "--db", databasePath)
	if humanPromotion.exitCode != 0 || humanPromotion.stderr != "" ||
		!strings.Contains(humanPromotion.stdout, "Done:") ||
		!strings.Contains(humanPromotion.stdout, "Promoted:") ||
		!strings.Contains(humanPromotion.stdout, planned.Title+" → Review") ||
		strings.Contains(humanPromotion.stdout, "already at last stage") {
		t.Errorf("human promotion = %#v, want advancing task and project narration", humanPromotion)
	}

	lastJSON := decodeTask(t, runJSON(
		"add", "Last stage JSON", "--project", fmt.Sprint(planned.ID), "--promotes",
	))
	lastCompletion := decodeJSON[task.Completion](t, runJSON(
		"done", fmt.Sprint(lastJSON.ID),
	), "last-stage completion")
	if lastCompletion.Task.Status != "done" || lastCompletion.PromotedProject != nil {
		t.Errorf("last-stage completion = %#v, want done task and null promoted project", lastCompletion)
	}
	lastHuman := decodeTask(t, runJSON(
		"add", "Last stage human", "--project", fmt.Sprint(planned.ID), "--promotes",
	))
	lastNarration := runGSD(t, "done", fmt.Sprint(lastHuman.ID), "--db", databasePath)
	if lastNarration.exitCode != 0 || lastNarration.stderr != "" ||
		!strings.Contains(lastNarration.stdout, "Promoted:") ||
		!strings.Contains(lastNarration.stdout, "already at last stage") {
		t.Errorf("last-stage narration = %#v, want reported promotion no-op", lastNarration)
	}

	beforeDone := decodeJSON[board.Show](t, runJSON("board", "show", "Software"), "board before project done")
	assertShownProjectIDs(t, beforeDone, "Review", planned.ID)
	resolved := decodeProjectResolution(t, runJSON("project", "done", fmt.Sprint(planned.ID)))
	if resolved.Project.Status != "done" {
		t.Fatalf("resolved project = %#v, want done", resolved.Project)
	}
	afterDone := decodeJSON[board.Show](t, runJSON("board", "show", "Software"), "board after project done")
	if boardHasProject(afterDone, planned.ID) {
		t.Errorf("board after project done = %#v, want resolved project hidden", afterDone)
	}
	entries := decodeJSON[[]logbook.Entry](t, runJSON("logbook"), "logbook")
	if !logbookHasProject(entries, planned.ID, "done") {
		t.Errorf("logbook = %#v, want done project %d", entries, planned.ID)
	}
	reopened := decodeProject(t, runJSON("project", "reopen", fmt.Sprint(planned.ID)))
	if reopened.Status != "open" || reopened.StageID == nil {
		t.Fatalf("reopened project = %#v, want open project retaining stage", reopened)
	}
	afterProjectReopen := decodeJSON[board.Show](t, runJSON(
		"board", "show", "Software",
	), "board after project reopen")
	assertShownProjectIDs(t, afterProjectReopen, "Review", planned.ID)
	afterReopenEntries := decodeJSON[[]logbook.Entry](t, runJSON("logbook"), "logbook after reopen")
	if logbookHasProject(afterReopenEntries, planned.ID, "done") {
		t.Errorf("logbook after project reopen = %#v, want reopened project absent", afterReopenEntries)
	}

	assertJSONError(t, runJSON("board", "show", "Missing"), apperr.NotFound)
	assertJSONError(t, runJSON("projects", "add", "Unknown board", "--board", "Missing"), apperr.NotFound)
	assertJSONError(t, runJSON(
		"add", "Unknown stage", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Absent",
	), apperr.NotFound)
	assertJSONError(t, runJSON(
		"add", "Foreign stage", "--project", fmt.Sprint(planned.ID), "--defer-stage", "Shipping",
	), apperr.InvalidArgument)
	assertJSONError(t, runJSON(
		"project", "move", fmt.Sprint(planned.ID), "Absent",
	), apperr.NotFound)

	for index, args := range [][]string{
		{"board", "--help"},
		{"edit", "1", "--defer-stage", "Doing", "--no-defer-stage", "--json"},
	} {
		unusedPath := filepath.Join(workDir, "board-no-open", fmt.Sprintf("%d.db", index))
		result := runGSD(t, append(args, "--db", unusedPath)...)
		if index == 0 {
			if result.exitCode != 0 || result.stdout == "" || result.stderr != "" {
				t.Errorf("board help = %#v, want stdout-only success", result)
			}
		} else if result.exitCode != 2 || result.stdout != "" || result.stderr == "" {
			t.Errorf("task defer-stage parse failure = %#v, want stderr-only usage exit", result)
		}
		if _, err := os.Stat(unusedPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("non-behavioral command database stat error = %v, want not exist", err)
		}
	}
}

func assertStageTitles(t *testing.T, description string, stages []board.Stage, want ...string) {
	t.Helper()
	got := make([]string, len(stages))
	for index, stage := range stages {
		got[index] = stage.Title
		if stage.Position != int64(index) {
			t.Errorf("%s stage %q position = %d, want %d", description, stage.Title, stage.Position, index)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s stage titles = %v, want %v", description, got, want)
	}
}

func assertShownStageTitles(t *testing.T, description string, shown board.Show, want ...string) {
	t.Helper()
	stages := make([]board.Stage, len(shown.Stages))
	for index, stage := range shown.Stages {
		stages[index] = stage.Stage
	}
	assertStageTitles(t, description, stages, want...)
}

func shownStage(t *testing.T, shown board.Show, title string) board.ShownStage {
	t.Helper()
	for _, stage := range shown.Stages {
		if stage.Title == title {
			return stage
		}
	}
	t.Fatalf("board show = %#v, want stage %q", shown, title)
	return board.ShownStage{}
}

func assertShownProjectIDs(t *testing.T, shown board.Show, stageTitle string, want ...int64) {
	t.Helper()
	stage := shownStage(t, shown, stageTitle)
	got := make([]int64, len(stage.Projects))
	for index, current := range stage.Projects {
		got[index] = current.ID
	}
	if !slices.Equal(got, want) {
		t.Errorf("stage %q project IDs = %v, want %v", stageTitle, got, want)
	}
}

func assertTaskVisibility(t *testing.T, rows []task.ViewTask, id int64, want bool) {
	t.Helper()
	got := false
	for _, row := range rows {
		if row.ID == id {
			got = true
			break
		}
	}
	if got != want {
		t.Errorf("available task IDs contain %d = %t, want %t; rows = %#v", id, got, want, rows)
	}
}

func boardHasProject(shown board.Show, id int64) bool {
	for _, stage := range shown.Stages {
		for _, current := range stage.Projects {
			if current.ID == id {
				return true
			}
		}
	}
	return false
}

func logbookHasProject(entries []logbook.Entry, id int64, status string) bool {
	for _, entry := range entries {
		if entry.Kind == "project" && entry.ID == id && entry.Status == status {
			return true
		}
	}
	return false
}
