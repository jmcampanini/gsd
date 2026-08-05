package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const (
	latteGreenSGR  = "38;2;64;160;43"
	frappeGreenSGR = "38;2;166;209;137"
	frappeRedSGR   = "38;2;231;130;132"
)

func renderHuman(
	t *testing.T,
	profile colorprofile.Profile,
	render func(humanOutput) error,
) string {
	t.Helper()
	return renderHumanOnBackground(t, profile, true, render)
}

func renderHumanOnBackground(
	t *testing.T,
	profile colorprofile.Profile,
	dark bool,
	render func(humanOutput) error,
) string {
	t.Helper()
	var output bytes.Buffer
	human := newHumanOutput(
		&colorprofile.Writer{Forward: &output, Profile: profile},
		dark,
		"2026-08-02",
	)
	if err := render(human); err != nil {
		t.Fatalf("render human output: %v", err)
	}
	return output.String()
}

func TestHumanCollectionStructureIsModeIndependent(t *testing.T) {
	t.Parallel()

	overdue := "2026-08-01"
	future := "2026-08-05"
	tasks := []task.Task{
		{ID: 2, Title: "Open", Status: "open", DueOn: &overdue},
		{ID: 12, Title: "Finished", Status: "done", DueOn: &overdue},
		{ID: 20, Title: "Later", Status: "open", DueOn: &future},
	}
	render := func(output humanOutput) error { return output.writeTaskList(tasks) }
	plain := renderHuman(t, colorprofile.NoTTY, render)
	styled := renderHuman(t, colorprofile.TrueColor, render)

	want := "  id  title     status  dates\n" +
		"   2  Open      open    due 2026-08-01\n" +
		"  12  Finished  done    due 2026-08-01\n" +
		"  20  Later     open    due 2026-08-05\n"
	if plain != want {
		t.Errorf("plain collection = %q, want %q", plain, want)
	}
	if !strings.Contains(styled, "\x1b[") {
		t.Errorf("styled collection = %q, want ANSI", styled)
	}
	if stripped := ansi.Strip(styled); stripped != plain {
		t.Errorf("stripped styled collection = %q, want structure %q", stripped, plain)
	}
}

func TestHumanOutputSelectsBackgroundAdaptiveAccents(t *testing.T) {
	t.Parallel()

	render := func(output humanOutput) error {
		return output.writeTaskMutation(verbDone, task.Task{ID: 5, Title: "Take out recycling"})
	}
	light := renderHumanOnBackground(t, colorprofile.TrueColor, false, render)
	dark := renderHumanOnBackground(t, colorprofile.TrueColor, true, render)
	if !strings.Contains(light, latteGreenSGR) {
		t.Errorf("light mutation = %q, want Catppuccin Latte green", light)
	}
	if !strings.Contains(dark, frappeGreenSGR) {
		t.Errorf("dark mutation = %q, want Catppuccin Frappé green", dark)
	}
}

func TestUrgentDatesApplyOnlyToOpenTasks(t *testing.T) {
	t.Parallel()

	due := "2026-08-02"
	styled := renderHuman(t, colorprofile.TrueColor, func(output humanOutput) error {
		return output.writeTaskList([]task.Task{
			{ID: 1, Title: "Urgent", Status: "open", DueOn: &due},
			{ID: 2, Title: "Settled", Status: "done", DueOn: &due},
		})
	})
	var urgentRow, settledRow string
	for _, line := range strings.Split(styled, "\n") {
		if strings.Contains(line, "Urgent") {
			urgentRow = line
		}
		if strings.Contains(line, "Settled") {
			settledRow = line
		}
	}
	if !strings.Contains(urgentRow, "\x1b[1;") || !strings.Contains(urgentRow, frappeRedSGR) {
		t.Errorf("open due row = %q, want bold Frappé red", urgentRow)
	}
	if settledRow == "" || strings.Contains(settledRow, "\x1b[1;") {
		t.Errorf("resolved due row = %q, want no bold urgency", settledRow)
	}
}

func TestFinalGlyphVocabulary(t *testing.T) {
	t.Parallel()

	archivedAt := "2026-08-01T12:00:00Z"
	tests := []struct {
		name   string
		prefix string
		render func(humanOutput) error
	}{
		{name: "task open", prefix: "• 1  Task", render: func(o humanOutput) error {
			return o.writeTask(task.Task{ID: 1, Title: "Task", Status: "open"})
		}},
		{name: "task done", prefix: "✓ 1  Task", render: func(o humanOutput) error {
			return o.writeTask(task.Task{ID: 1, Title: "Task", Status: "done"})
		}},
		{name: "task cancelled", prefix: "✗ 1  Task", render: func(o humanOutput) error {
			return o.writeTask(task.Task{ID: 1, Title: "Task", Status: "cancelled"})
		}},
		{name: "project open", prefix: "◆ 2  Project", render: func(o humanOutput) error {
			return o.writeProject(project.Project{ID: 2, Title: "Project", Status: "open"})
		}},
		{name: "project done", prefix: "✓ 2  Project", render: func(o humanOutput) error {
			return o.writeProject(project.Project{ID: 2, Title: "Project", Status: "done"})
		}},
		{name: "project cancelled", prefix: "✗ 2  Project", render: func(o humanOutput) error {
			return o.writeProject(project.Project{ID: 2, Title: "Project", Status: "cancelled"})
		}},
		{name: "area active", prefix: "● 3  Area", render: func(o humanOutput) error {
			return o.writeArea(area.Area{ID: 3, Title: "Area"})
		}},
		{name: "area archived", prefix: "✗ 3  Area", render: func(o humanOutput) error {
			return o.writeArea(area.Area{ID: 3, Title: "Area", ArchivedAt: &archivedAt})
		}},
		{name: "add", prefix: "+ Added task 1", render: func(o humanOutput) error {
			return o.writeAddedTask(task.Task{ID: 1, Title: "Task"})
		}},
		{name: "delete", prefix: "− Deleted: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbDeleted, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "complete", prefix: "✓ Done: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbDone, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "cancel", prefix: "✗ Cancelled: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbCancelled, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "edit", prefix: "~ Edited: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbEdited, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "reopen", prefix: "~ Reopened: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbReopened, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "reorder", prefix: "~ Reordered: 1", render: func(o humanOutput) error {
			return o.writeTaskMutation(verbReordered, task.Task{ID: 1, Title: "Task"})
		}},
		{name: "archive", prefix: "✗ Archived: area 3", render: func(o humanOutput) error {
			return o.writeAreaMutation(verbArchived, area.Area{ID: 3, Title: "Area"})
		}},
		{name: "unarchive", prefix: "~ Unarchived: area 3", render: func(o humanOutput) error {
			return o.writeAreaMutation(verbUnarchived, area.Area{ID: 3, Title: "Area"})
		}},
		{name: "tag", prefix: "+# Tagged: task 1", render: func(o humanOutput) error {
			return o.writeTaskTagging(verbTagged, task.Tagging{Task: task.Task{ID: 1}, TagTitles: []string{"Home"}})
		}},
		{name: "untag", prefix: "−# Untagged: task 1", render: func(o humanOutput) error {
			return o.writeTaskTagging(verbUntagged, task.Tagging{Task: task.Task{ID: 1}, TagTitles: []string{"Home"}})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := renderHuman(t, colorprofile.NoTTY, test.render)
			if !strings.HasPrefix(got, test.prefix) {
				t.Errorf("output = %q, want prefix %q", got, test.prefix)
			}
		})
	}
}

func TestCascadeUsesStandardTreeBranches(t *testing.T) {
	t.Parallel()

	got := renderHuman(t, colorprofile.NoTTY, func(output humanOutput) error {
		return output.writeProjectDeletion(project.Deletion{
			Project: project.Project{ID: 4, Title: "Kitchen"},
			DeletedTasks: []task.Task{
				{ID: 18, Title: "First"},
				{ID: 19, Title: "Second"},
				{ID: 21, Title: "Third"},
			},
		})
	})
	want := "− Deleted: project 4  Kitchen\n" +
		"Deleted 3 tasks:\n" +
		"  ├ 18  First\n" +
		"  ├ 19  Second\n" +
		"  └ 21  Third\n"
	if got != want {
		t.Errorf("cascade = %q, want %q", got, want)
	}
}

func TestShowUsesGlyphHeadlineAndLowercaseOutline(t *testing.T) {
	t.Parallel()

	due := "2026-08-15"
	got := renderHuman(t, colorprofile.NoTTY, func(output humanOutput) error {
		return output.writeTask(task.Task{
			ID:        12,
			Title:     "Book dentist appointment",
			Note:      "first line\nsecond line",
			DueOn:     &due,
			Status:    "open",
			CreatedAt: "2026-08-01T12:00:00Z",
			Tags:      []string{"errand", "health"},
		})
	})
	for _, fragment := range []string{
		"• 12  Book dentist appointment\n",
		"    note          first line\n                  second line\n",
		"    due on        2026-08-15\n",
		"    tags          #errand #health\n",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("show = %q, want fragment %q", got, fragment)
		}
	}
	if strings.Contains(got, "    id") || strings.Contains(got, "    title") {
		t.Errorf("show = %q, want ID and title only in headline", got)
	}
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if strings.HasSuffix(line, " ") {
			t.Errorf("show line = %q, want no trailing spaces", line)
		}
	}
}
