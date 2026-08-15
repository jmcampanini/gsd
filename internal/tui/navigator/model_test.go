package navigator

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
)

type fakeTasks struct {
	inboxResponses     [][]task.ViewTask
	availableResponses [][]task.ViewTask
	listResponses      [][]task.Task
	showResponses      []task.Task
	inboxErr           error
	availableErr       error
	listErr            error
	showErr            error
	inboxCalls         int
	availableCalls     int
	listCalls          int
	showCalls          int
	listOptions        []task.ListOptions
	showIDs            []int64
}

func (f *fakeTasks) Inbox(context.Context) ([]task.ViewTask, error) {
	response := responseAt(f.inboxResponses, f.inboxCalls)
	f.inboxCalls++
	return response, f.inboxErr
}

func (f *fakeTasks) Available(context.Context) ([]task.ViewTask, error) {
	response := responseAt(f.availableResponses, f.availableCalls)
	f.availableCalls++
	return response, f.availableErr
}

func (f *fakeTasks) List(_ context.Context, options task.ListOptions) ([]task.Task, error) {
	response := responseAt(f.listResponses, f.listCalls)
	f.listCalls++
	f.listOptions = append(f.listOptions, options)
	return response, f.listErr
}

func (f *fakeTasks) Show(_ context.Context, id int64) (task.Task, error) {
	response := valueAt(f.showResponses, f.showCalls)
	f.showCalls++
	f.showIDs = append(f.showIDs, id)
	return response, f.showErr
}

type fakeProjects struct {
	responses     [][]project.Project
	showResponses []project.Detail
	err           error
	showErr       error
	calls         int
	showCalls     int
	options       []project.ListOptions
	showIDs       []int64
}

func (f *fakeProjects) List(_ context.Context, options project.ListOptions) ([]project.Project, error) {
	response := responseAt(f.responses, f.calls)
	f.calls++
	f.options = append(f.options, options)
	return response, f.err
}

func (f *fakeProjects) Show(_ context.Context, id int64) (project.Detail, error) {
	response := valueAt(f.showResponses, f.showCalls)
	f.showCalls++
	f.showIDs = append(f.showIDs, id)
	return response, f.showErr
}

type fakeAreas struct {
	items         []area.Area
	showResponses []area.Area
	err           error
	showErr       error
	calls         int
	showCalls     int
	options       []area.ListOptions
	showIDs       []int64
}

func (f *fakeAreas) List(_ context.Context, options area.ListOptions) ([]area.Area, error) {
	f.calls++
	f.options = append(f.options, options)
	return f.items, f.err
}

func (f *fakeAreas) Show(_ context.Context, id int64) (area.Area, error) {
	response := valueAt(f.showResponses, f.showCalls)
	f.showCalls++
	f.showIDs = append(f.showIDs, id)
	return response, f.showErr
}

type fakeBoards struct {
	items         []board.ListedBoard
	showResponses []board.Show
	err           error
	showErr       error
	calls         int
	showCalls     int
	showIDs       []int64
}

func (f *fakeBoards) List(context.Context) ([]board.ListedBoard, error) {
	f.calls++
	return f.items, f.err
}

func (f *fakeBoards) ShowByID(_ context.Context, id int64) (board.Show, error) {
	response := valueAt(f.showResponses, f.showCalls)
	f.showCalls++
	f.showIDs = append(f.showIDs, id)
	return response, f.showErr
}

type fakeLogbook struct {
	items []logbook.Entry
	err   error
	calls int
}

func (f *fakeLogbook) List(context.Context) ([]logbook.Entry, error) {
	f.calls++
	return f.items, f.err
}

func testDependencies() (
	Dependencies,
	*fakeTasks,
	*fakeProjects,
	*fakeAreas,
	*fakeBoards,
	*fakeLogbook,
) {
	tasks := &fakeTasks{}
	projects := &fakeProjects{}
	areas := &fakeAreas{}
	boards := &fakeBoards{}
	entries := &fakeLogbook{}
	return Dependencies{
		Tasks:    tasks,
		Projects: projects,
		Areas:    areas,
		Boards:   boards,
		Logbook:  entries,
	}, tasks, projects, areas, boards, entries
}

func TestRootNavigationPushPopAndQuit(t *testing.T) {
	dependencies, tasks, _, areas, boards, entries := testDependencies()
	wantRoot := " gsd\n\n▌ Inbox\n  Available\n  Logbook\n  Boards\n  Areas\n\n j/k move · ⏎ open · esc quit\n"
	initial := newModel(context.Background(), dependencies, false, time.UTC)
	if got := initial.View().Content; got != wantRoot {
		t.Fatalf("root view = %q, want %q", got, wantRoot)
	}

	loadCalls := []func() int{
		func() int { return tasks.inboxCalls },
		func() int { return tasks.availableCalls },
		func() int { return entries.calls },
		func() int { return boards.calls },
		func() int { return areas.calls },
	}
	for index, wantKind := range []viewKind{viewInbox, viewAvailable, viewLogbook, viewBoards, viewAreas} {
		current := newModel(context.Background(), dependencies, false, time.UTC)
		current = pressTimes(t, current, "j", index)
		updated, load := press(t, current, "enter")
		if updated.top().key.kind != wantKind {
			t.Fatalf("root row %d pushed kind %d, want %d", index, updated.top().key.kind, wantKind)
		}
		if got := updated.View().Content; !strings.Contains(got, "  loading…") {
			t.Errorf("loading view = %q, want dim loading row", got)
		}
		before := loadCalls[index]()
		updated = deliver(t, updated, load)
		if got := loadCalls[index](); got != before+1 {
			t.Errorf("root row %d loader calls = %d, want %d", index, got, before+1)
		}
		updated, command := press(t, updated, "esc")
		if command != nil || len(updated.stack) != 1 {
			t.Errorf("Esc child stack/command = %d/%v, want root", len(updated.stack), command)
		}
	}

	current := initial
	current, _ = press(t, current, "k")
	if current.top().cursor != 0 {
		t.Errorf("cursor after k at top = %d, want 0", current.top().cursor)
	}
	current = pressTimes(t, current, "down", 9)
	if current.top().cursor != 4 {
		t.Errorf("cursor after down past end = %d, want 4", current.top().cursor)
	}
	current = pressTimes(t, current, "up", 9)
	if current.top().cursor != 0 {
		t.Errorf("cursor after up past start = %d, want 0", current.top().cursor)
	}

	_, rootQuit := press(t, current, "esc")
	assertQuit(t, rootQuit)
	child, _ := press(t, current, "enter")
	for _, key := range []string{"q", "ctrl+c"} {
		_, anywhereQuit := press(t, child, key)
		assertQuit(t, anywhereQuit)
	}
	back, command := press(t, child, "h")
	if command != nil || len(back.stack) != 1 {
		t.Errorf("h from child stack/command = %d/%v, want root", len(back.stack), command)
	}
}

func TestHorizontalVimKeysDrillInAndOutAtEveryImplementedLevel(t *testing.T) {
	dependencies, _, projects, areas, _, _ := testDependencies()
	areaID := int64(7)
	areas.items = []area.Area{{ID: areaID, Title: "Home"}}
	areas.showResponses = []area.Area{{ID: areaID, Title: "Home"}}
	projects.responses = [][]project.Project{{{
		ID: 11, AreaID: &areaID, Title: "Kitchen reno", Status: "open",
	}}}
	projects.showResponses = []project.Detail{{
		Project: project.Project{ID: 11, AreaID: &areaID, Title: "Kitchen reno", Status: "open"},
	}}

	current := pressTimes(t, newModel(context.Background(), dependencies, false, time.UTC), "j", 4)
	current, load := press(t, current, "l")
	current = deliver(t, current, load)
	if current.top().key.kind != viewAreas {
		t.Fatalf("root l key = %#v, want areas collection", current.top().key)
	}

	current, load = press(t, current, "l")
	current = deliver(t, current, load)
	if current.top().key != (viewKey{kind: viewArea, id: areaID}) {
		t.Fatalf("collection l key = %#v, want area 7", current.top().key)
	}

	current, _ = press(t, current, "j")
	current, load = press(t, current, "l")
	current = deliver(t, current, load)
	if current.top().key != (viewKey{kind: viewProject, id: 11}) {
		t.Fatalf("container l key = %#v, want project 11", current.top().key)
	}

	for _, wantKind := range []viewKind{viewArea, viewAreas} {
		current, load = press(t, current, "h")
		current = deliver(t, current, load)
		if current.top().key.kind != wantKind {
			t.Fatalf("h key kind = %d, want %d", current.top().key.kind, wantKind)
		}
	}
	current, command := press(t, current, "h")
	if command != nil || current.top().key.kind != viewRoot {
		t.Fatalf("collection h stack/command = %d/%v, want root", len(current.stack), command)
	}
	current, command = press(t, current, "h")
	if command != nil || len(current.stack) != 1 {
		t.Errorf("root h stack/command = %d/%v, want inert", len(current.stack), command)
	}
}

func TestViewReentryReloadsAndIgnoresStaleResponses(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.inboxResponses = [][]task.ViewTask{
		{{Task: task.Task{ID: 1, Title: "stale"}}},
		{{Task: task.Task{ID: 2, Title: "fresh"}}},
		{{Task: task.Task{ID: 3, Title: "newest"}}},
	}
	current := newModel(context.Background(), dependencies, false, time.UTC)
	firstEntry, staleLoad := press(t, current, "enter")
	staleResult := staleLoad()
	root, _ := press(t, firstEntry, "esc")
	secondEntry, freshLoad := press(t, root, "enter")
	secondEntry = deliver(t, secondEntry, freshLoad)
	if got := secondEntry.View().Content; !strings.Contains(got, "fresh") {
		t.Fatalf("second entry view = %q, want fresh response", got)
	}
	updated, _ := secondEntry.Update(staleResult)
	secondEntry = updated.(model)
	if got := secondEntry.View().Content; strings.Contains(got, "stale") || !strings.Contains(got, "fresh") {
		t.Fatalf("view after stale response = %q, want fresh response unchanged", got)
	}

	root, _ = press(t, secondEntry, "esc")
	thirdEntry, newestLoad := press(t, root, "enter")
	thirdEntry = deliver(t, thirdEntry, newestLoad)
	if tasks.inboxCalls != 3 || !strings.Contains(thirdEntry.View().Content, "newest") {
		t.Errorf("re-entry calls/view = %d/%q, want third read", tasks.inboxCalls, thirdEntry.View().Content)
	}
}

func TestCursorRestoresByIdentityAndClampsWhenSelectionDisappears(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.inboxResponses = [][]task.ViewTask{
		{
			{Task: task.Task{ID: 1, Title: "one"}},
			{Task: task.Task{ID: 2, Title: "two"}},
			{Task: task.Task{ID: 3, Title: "three"}},
		},
		{
			{Task: task.Task{ID: 3, Title: "three"}},
			{Task: task.Task{ID: 2, Title: "two"}},
			{Task: task.Task{ID: 1, Title: "one"}},
		},
		{{Task: task.Task{ID: 4, Title: "four"}}},
	}
	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 0)
	current, _ = press(t, current, "j")
	root, _ := press(t, current, "esc")
	current = enterRootRow(t, root, 0)
	if current.top().cursor != 1 || !strings.Contains(selectedLine(current.View().Content), "two") {
		t.Fatalf("restored cursor/view = %d/%q, want task 2 by identity", current.top().cursor, current.View().Content)
	}
	current, _ = press(t, current, "j")
	root, _ = press(t, current, "esc")
	current = enterRootRow(t, root, 0)
	if current.top().cursor != 0 || !strings.Contains(selectedLine(current.View().Content), "four") {
		t.Fatalf("clamped cursor/view = %d/%q, want sole row", current.top().cursor, current.View().Content)
	}
}

func TestLoadFailureIsSanitizedInlineAndNavigationRemainsAlive(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.inboxErr = errors.New("database\x1b[31m failed\nretry")
	current := newModel(context.Background(), dependencies, true, time.UTC)
	current, load := press(t, current, "enter")
	current = deliver(t, current, load)
	redAccent := lipgloss.NewStyle().Foreground(tui.ThemeForBackground(true).Red).Render("! ")
	wantError := redAccent + `database\x1b[31m failed\nretry`
	if got := current.View().Content; !strings.Contains(got, wantError+"\n") {
		t.Errorf("error view = %q, want inline %q", got, wantError)
	}
	current, command := press(t, current, "esc")
	if command != nil || len(current.stack) != 1 {
		t.Errorf("Esc from failed view stack/command = %d/%v, want live root", len(current.stack), command)
	}
}

func TestTaskAndLogbookRowsUseRecordShape(t *testing.T) {
	dependencies, tasks, _, _, _, entries := testDependencies()
	due := "2026-08-08"
	deferUntil := "2026-08-09"
	stage := "Review\tNow"
	listedTask := task.ViewTask{Task: task.Task{
		ID:              12,
		Title:           "Ship\x1b",
		DueOn:           &due,
		DeferUntil:      &deferUntil,
		DeferStageTitle: &stage,
		Promotes:        true,
	}}
	tasks.inboxResponses = [][]task.ViewTask{{listedTask}}
	tasks.availableResponses = [][]task.ViewTask{{listedTask}}
	for _, rootIndex := range []int{0, 1} {
		current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), rootIndex)
		view := current.View().Content
		fragment := `• Ship\x1b ↑  due 2026-08-08 defer 2026-08-09 defer→Review\tNow`
		if !strings.Contains(view, fragment) {
			t.Errorf("task view %q missing %q", view, fragment)
		}
		if strings.Contains(view, "12") {
			t.Errorf("task view %q, want no id in rows", view)
		}
	}

	entries.items = []logbook.Entry{{
		Kind:       "task",
		ID:         7,
		Title:      "Done thing",
		Status:     "done",
		ResolvedAt: "2026-08-08T02:30:00Z",
	}}
	location := time.FixedZone("west", -7*60*60)
	current := enterRootRow(t, newModel(context.Background(), dependencies, false, location), 2)
	view := current.View().Content
	if !strings.Contains(view, "✓ Done thing  task  2026-08-07") {
		t.Errorf("logbook view %q, want status glyph, title, dim kind and date", view)
	}
}

func TestBoardAndAreaRowsUseServiceOrderContractsAndApprovedShape(t *testing.T) {
	dependencies, _, _, areas, boards, _ := testDependencies()
	boards.items = []board.ListedBoard{
		{Board: board.Board{ID: 1, Title: "First", Position: 1}},
		{
			Board: board.Board{ID: 2, Title: "Second", Position: 2},
			Stages: []board.Stage{
				{ID: 20, Title: "Research", Position: 1},
				{ID: 21, Title: "Doing", Position: 2},
				{ID: 22, Title: "Review", Position: 3},
			},
		},
	}
	boardView := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 3).View().Content
	if strings.Index(boardView, "First") > strings.Index(boardView, "Second") {
		t.Errorf("board view order = %q, want position order", boardView)
	}
	if !strings.Contains(boardView, "Second  Research → Doing → Review") {
		t.Errorf("board view %q, want title then stage chain", boardView)
	}

	areas.items = []area.Area{
		{ID: 4, Title: "Home", Position: 1},
		{ID: 8, Title: "Work", Position: 2},
	}
	areaView := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4).View().Content
	if len(areas.options) != 1 || areas.options[0].Slice != area.ListSliceActive {
		t.Fatalf("area List options = %#v, want active slice", areas.options)
	}
	if strings.Index(areaView, "Home") > strings.Index(areaView, "Work") ||
		strings.Index(areaView, "Work") > strings.Index(areaView, "(no area)") {
		t.Errorf("area view order = %q, want active positions then pseudo-row", areaView)
	}
	for _, fragment := range []string{"● Home", "● Work", "(no area)"} {
		if !strings.Contains(areaView, fragment) {
			t.Errorf("area view %q missing %q", areaView, fragment)
		}
	}
}

func TestEmptyListsMatchCLIAndAreasRetainsPseudoRow(t *testing.T) {
	dependencies, _, _, _, _, _ := testDependencies()
	labels := []string{"Inbox", "Available", "Logbook", "Boards"}
	for rootIndex, label := range labels {
		current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), rootIndex)
		want := " gsd  " + label + "\n\n\n j/k move · ⏎ open · esc back\n"
		if got := current.View().Content; got != want {
			t.Errorf("empty root row %d view = %q, want framed empty view %q", rootIndex, got, want)
		}
	}
	areas := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4)
	if got := areas.View().Content; !strings.Contains(got, "▌ ○ (no area)") {
		t.Errorf("empty areas view = %q, want selected hollow pseudo-row", got)
	}
}

func TestFilterNarrowsPerKeystrokeAndEscRestoresRowsAndCursor(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.availableResponses = [][]task.ViewTask{{
		{Task: task.Task{ID: 1, Title: "Call plumber", Status: "open"}},
		{Task: task.Task{ID: 2, Title: "Plan meal", Status: "open"}},
	}}
	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 1)
	current, _ = press(t, current, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Plan meal") {
		t.Fatalf("selection before filter = %q, want Plan meal", selected)
	}

	current, _ = press(t, current, "/")
	for _, key := range []string{"p", "l", "m"} {
		current, _ = press(t, current, key)
		if view := current.View().Content; !strings.Contains(view, "Call plumber") {
			t.Fatalf("view after %q = %q, want incremental plumber match", key, view)
		}
	}
	current, _ = press(t, current, "b")
	filtered := current.View()
	if !current.top().filter.editing || !strings.Contains(filtered.Content, "/ plmb") {
		t.Fatalf("active filter = %#v/%q, want editing band", current.top().filter, filtered.Content)
	}
	if !strings.Contains(filtered.Content, "Call plumber") || strings.Contains(filtered.Content, "Plan meal") {
		t.Fatalf("filtered view = %q, want only Call plumber", filtered.Content)
	}
	if selected := selectedLine(filtered.Content); !strings.Contains(selected, "Call plumber") {
		t.Fatalf("filtered selection = %q, want matched row", selected)
	}
	if filtered.Cursor == nil {
		t.Fatal("editing cursor = nil, want terminal cursor in input band")
	}
	for range 4 {
		current, _ = press(t, current, "backspace")
	}
	if view := current.View().Content; !strings.Contains(view, "Call plumber") || !strings.Contains(view, "Plan meal") {
		t.Fatalf("blank filter view = %q, want unfiltered rows while editing", view)
	}
	for _, key := range []string{"p", "l", "m", "b"} {
		current, _ = press(t, current, key)
	}

	current, command := press(t, current, "esc")
	if command != nil || !current.top().filter.enabled || current.top().filter.editing {
		t.Fatalf("Esc while editing filter/command = %#v/%v, want committed filtered navigation", current.top().filter, command)
	}
	committed := current.View()
	if committed.Cursor != nil || !strings.Contains(committed.Content, "Call plumber") || strings.Contains(committed.Content, "Plan meal") {
		t.Fatalf("committed view = %q, want retained filter without an input cursor", committed.Content)
	}
	current, command = press(t, current, "esc")
	if command != nil || current.top().filter.enabled {
		t.Fatalf("second Esc filter state/command = %#v/%v, want cleared without navigation", current.top().filter, command)
	}
	restored := current.View().Content
	if !strings.Contains(restored, "Call plumber") || !strings.Contains(restored, "Plan meal") {
		t.Fatalf("restored view = %q, want every source row", restored)
	}
	if selected := selectedLine(restored); !strings.Contains(selected, "Plan meal") {
		t.Fatalf("restored selection = %q, want original cursor", selected)
	}
	current, command = press(t, current, "esc")
	if command != nil || current.top().key.kind != viewRoot {
		t.Fatalf("third Esc key/command = %#v/%v, want parent", current.top().key, command)
	}
}

func TestArrowKeysCommitTheFilterAndKeepMovingWithinMatches(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.availableResponses = [][]task.ViewTask{{
		{Task: task.Task{ID: 1, Title: "Call plumber", Status: "open"}},
		{Task: task.Task{ID: 2, Title: "Plan meal", Status: "open"}},
		{Task: task.Task{ID: 3, Title: "Buy cabinet pulls", Status: "open"}},
		{Task: task.Task{ID: 4, Title: "Read book", Status: "open"}},
	}}
	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 1)
	current, _ = press(t, current, "/")
	for _, key := range []string{"p", "l"} {
		current, _ = press(t, current, key)
	}
	if view := current.View().Content; strings.Contains(view, "Read book") {
		t.Fatalf("filtered view = %q, want nonmatch hidden", view)
	}

	current, command := press(t, current, "down")
	if command != nil || !current.top().filter.enabled || current.top().filter.editing {
		t.Fatalf("down while editing filter/command = %#v/%v, want committed filtered navigation", current.top().filter, command)
	}
	if current.View().Cursor != nil {
		t.Fatal("committed view cursor != nil, want focus on the list")
	}
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Plan meal") {
		t.Fatalf("selection after down = %q, want the next match", selected)
	}
	current, _ = press(t, current, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Buy cabinet pulls") {
		t.Fatalf("selection after j = %q, want movement within the matched set", selected)
	}

	current, _ = press(t, current, "/")
	if !current.top().filter.editing || current.top().filter.input.Value() != "pl" {
		t.Fatalf("filter after / = %#v, want editing with the retained query", current.top().filter)
	}
	current, _ = press(t, current, "up")
	if selected := selectedLine(current.View().Content); current.top().filter.editing || !strings.Contains(selected, "Plan meal") {
		t.Fatalf("selection after up = %q (editing %v), want a committed move up", selected, current.top().filter.editing)
	}

	current, command = press(t, current, "enter")
	if command == nil || current.top().key != (viewKey{kind: viewTaskDetail, id: 2}) {
		t.Fatalf("enter key/command = %#v/%v, want the selected match opened", current.top().key, command)
	}
}

func TestCommittingABlankQueryClearsTheFilter(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.availableResponses = [][]task.ViewTask{{
		{Task: task.Task{ID: 1, Title: "Call plumber", Status: "open"}},
		{Task: task.Task{ID: 2, Title: "Plan meal", Status: "open"}},
	}}
	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 1)
	current, _ = press(t, current, "/")
	current, _ = press(t, current, "down")
	if top := current.top(); top.filter.enabled || top.cursor != 1 {
		t.Fatalf("blank down filter/cursor = %#v/%d, want cleared filter and moved cursor", top.filter, top.cursor)
	}
	current, _ = press(t, current, "/")
	current, command := press(t, current, "esc")
	if top := current.top(); command != nil || top.filter.enabled {
		t.Fatalf("blank Esc filter/command = %#v/%v, want cleared without navigation", top.filter, command)
	}
}

func TestFilterPreservesStructureAndDropsOnDescent(t *testing.T) {
	dependencies, tasks, projects, areas, _, _ := testDependencies()
	areas.items = []area.Area{{ID: 7, Title: "Home"}, {ID: 8, Title: "Work"}}
	areas.showResponses = []area.Area{{ID: 7, Title: "Home"}}
	projects.responses = [][]project.Project{{
		{ID: 11, AreaID: pointerTo(int64(7)), Title: "Kitchen reno", Status: "open"},
		{ID: 12, AreaID: pointerTo(int64(7)), Title: "Refresh entry", Status: "open"},
	}}
	projects.showResponses = []project.Detail{{Project: project.Project{ID: 11, Title: "Kitchen reno", Status: "open"}}}
	tasks.listResponses = [][]task.Task{{{ID: 21, AreaID: pointerTo(int64(7)), Title: "Water plants", Status: "open"}}}

	current := newModel(context.Background(), dependencies, false, time.UTC)
	current, _ = press(t, current, "/")
	for _, key := range []string{"a", "r", "e", "a", "s"} {
		current, _ = press(t, current, key)
	}
	if view := current.View().Content; !strings.Contains(view, "Areas") || strings.Contains(view, "Inbox") {
		t.Fatalf("filtered root = %q, want only Areas", view)
	}
	current, _ = press(t, current, "/")
	if current.View().Cursor != nil || current.top().filter.editing {
		t.Fatal("navigation-mode filter retained an editing cursor")
	}
	current, load := press(t, current, "l")
	current = deliver(t, current, load)
	if current.top().key.kind != viewAreas || current.stack[0].filter.enabled {
		t.Fatalf("root descent = %#v/filter %v, want Areas with parent filter dropped", current.top().key, current.stack[0].filter.enabled)
	}
	current, _ = press(t, current, "/")
	for _, key := range []string{"w", "o", "r", "k"} {
		current, _ = press(t, current, key)
	}
	if view := current.View().Content; !strings.Contains(view, "Work") || strings.Contains(view, "Home") {
		t.Fatalf("filtered collection = %q, want only Work", view)
	}
	current, _ = press(t, current, "esc")
	current, _ = press(t, current, "esc")

	current, load = press(t, current, "enter")
	current = deliver(t, current, load)
	current, _ = press(t, current, "/")
	for _, key := range []string{"h", "o", "m", "e"} {
		current, _ = press(t, current, key)
	}
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "● Home") {
		t.Fatalf("matching header selection = %q, want selectable Home header", selected)
	}
	if selected, ok := current.top().selectedRow(); !ok || selected.destination.kind != viewAreaDetail {
		t.Fatalf("matching header destination = %#v/%v, want area detail", selected.destination, ok)
	}
	current, _ = press(t, current, "esc")
	current, _ = press(t, current, "esc")
	current, _ = press(t, current, "/")
	for _, key := range []string{"r", "e", "n", "o"} {
		current, _ = press(t, current, key)
	}
	view := current.View().Content
	if !containsInOrder(view, "● Home", "projects", "Kitchen reno", "tasks") {
		t.Fatalf("filtered area structure = %q, want header and section headings preserved", view)
	}
	if strings.Contains(view, "Refresh entry") || strings.Contains(view, "Water plants") {
		t.Fatalf("filtered area = %q, want nonmatching rows hidden", view)
	}
	if selected := selectedLine(view); !strings.Contains(selected, "Kitchen reno") {
		t.Fatalf("filtered container selection = %q, want matched project rather than header", selected)
	}

	current, _ = press(t, current, "/")
	current, load = press(t, current, "l")
	current = deliver(t, current, load)
	if current.top().key != (viewKey{kind: viewProject, id: 11}) || current.stack[len(current.stack)-2].filter.enabled {
		t.Fatalf("filtered descent = %#v/parent filter %v, want project with filter dropped", current.top().key, current.stack[len(current.stack)-2].filter.enabled)
	}
}

func TestFilterModesHighlightMatchesAndLeaveDetailsInert(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	tasks.availableResponses = [][]task.ViewTask{{{Task: task.Task{ID: 1, Title: "Call plumber", Status: "open"}}}}
	current := enterRootRow(t, newModel(context.Background(), dependencies, true, time.UTC), 1)
	current, _ = press(t, current, "/")
	for _, key := range []string{"p", "l", "m", "b"} {
		current, _ = press(t, current, key)
	}
	theme := tui.ThemeForBackground(true)
	matchedSelected := lipgloss.NewStyle().
		Foreground(theme.Accent).
		Background(theme.InputBg).
		Bold(true).
		Render("p")
	if view := current.View().Content; !strings.Contains(view, matchedSelected) {
		t.Fatalf("colored filtered view = %q, want bold accent match on selection fill", view)
	}

	editing, _ := press(t, newModel(context.Background(), dependencies, false, time.UTC), "/")
	editing, command := press(t, editing, "q")
	if command != nil || editing.top().filter.input.Value() != "q" {
		t.Fatalf("q while editing command/query = %v/%q, want query text", command, editing.top().filter.input.Value())
	}
	editing, _ = press(t, editing, "/")
	_, command = press(t, editing, "q")
	assertQuit(t, command)

	detail := newModel(context.Background(), dependencies, false, time.UTC)
	detail.stack = append(detail.stack, frame{
		key:        viewKey{kind: viewTaskDetail, id: 1},
		loadedView: loadedView{detail: &detailView{kind: detailTask, id: 1, title: "Call plumber"}},
	})
	detail, command = press(t, detail, "/")
	if command != nil || detail.top().filter.enabled {
		t.Fatalf("slash on detail filter/command = %v/%v, want inert", detail.top().filter.enabled, command)
	}
}

func TestFilterInputViewportTracksDisplayWidthAndHorizontalOverflow(t *testing.T) {
	visible, cursor := filterInputViewport("a界b", 2, 8)
	if visible != "a界b" || cursor != 3 {
		t.Fatalf("wide viewport/cursor = %q/%d, want full value with cursor at display cell 3", visible, cursor)
	}
	visible, cursor = filterInputViewport("a👩‍💻b", 4, 8)
	if visible != "a👩‍💻b" || cursor != 3 {
		t.Fatalf("grapheme viewport/cursor = %q/%d, want full value with cursor at display cell 3", visible, cursor)
	}
	visible, cursor = filterInputViewport("abcdefgh", 8, 4)
	if visible != "efgh" || cursor != 4 {
		t.Fatalf("overflow end viewport/cursor = %q/%d, want efgh/4", visible, cursor)
	}
	visible, cursor = filterInputViewport("abcdefgh", 6, 4)
	if visible != "cdef" || cursor != 4 {
		t.Fatalf("overflow edited viewport/cursor = %q/%d, want cdef/4", visible, cursor)
	}
}

func TestSelectedRowUsesPickerFillWithBandsAndBackgroundCanChange(t *testing.T) {
	dependencies, _, _, _, _, _ := testDependencies()
	current := newModel(context.Background(), dependencies, true, time.UTC)
	if current.Init() == nil {
		t.Fatal("colored Init command = nil, want background detection")
	}
	selectedRow := func(theme tui.Theme, body string) string {
		edge := lipgloss.NewStyle().Foreground(theme.Accent).Background(theme.InputBg).Render("▌ ")
		fill := lipgloss.NewStyle().Foreground(theme.Text).Background(theme.InputBg).Render(body)
		return edge + fill
	}
	darkTheme := tui.ThemeForBackground(true)
	if got := current.View().Content; !strings.Contains(got, selectedRow(darkTheme, "Inbox")) {
		t.Errorf("dark selected row = %q, want accent edge with row fill", got)
	}
	badge := lipgloss.NewStyle().
		Background(darkTheme.Accent).
		Foreground(darkTheme.AccentText).
		Render(" gsd ")
	if got := current.View().Content; !strings.Contains(got, badge) {
		t.Errorf("dark root view = %q, want gsd badge in top band", got)
	}
	updated, _ := current.Update(tea.BackgroundColorMsg{Color: lipgloss.Color("#ffffff")})
	current = updated.(model)
	lightTheme := tui.ThemeForBackground(false)
	if got := current.View().Content; !strings.Contains(got, selectedRow(lightTheme, "Inbox")) {
		t.Errorf("light selected row = %q, want detected theme fill", got)
	}
	current, _ = press(t, current, "j")
	view := current.View().Content
	if !strings.Contains(view, "\n  Inbox\n") || !strings.Contains(view, selectedRow(lightTheme, "Available")) {
		t.Errorf("moved selection view = %q, want fill only on selection", view)
	}

	loading, _ := press(t, newModel(context.Background(), dependencies, true, time.UTC), "enter")
	dimLoading := lipgloss.NewStyle().Foreground(darkTheme.Dim).Render("loading…")
	if got := loading.View().Content; !strings.Contains(got, "  "+dimLoading+"\n") {
		t.Errorf("colored loading view = %q, want dim loading row", got)
	}
}

func TestUrgencyDatesAndLogbookGlyphsRenderAccents(t *testing.T) {
	dependencies, tasks, _, _, _, entries := testDependencies()
	upcoming := "2026-09-01"
	overdue := "2026-08-10"
	tasks.availableResponses = [][]task.ViewTask{{
		{Task: task.Task{ID: 1, Title: "Passport", Status: "open", DueOn: &upcoming}},
		{Task: task.Task{ID: 2, Title: "Taxes", Status: "open", DueOn: &overdue}},
	}}
	theme := tui.ThemeForBackground(true)

	current := newModel(context.Background(), dependencies, true, time.UTC)
	current.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC) }
	view := enterRootRow(t, current, 1).View().Content
	selectedDue := lipgloss.NewStyle().
		Background(theme.InputBg).
		Foreground(theme.Yellow).
		Render("due 2026-09-01")
	if !strings.Contains(view, selectedDue) {
		t.Errorf("available view = %q, want yellow due date on the selected fill", view)
	}
	overdueDue := lipgloss.NewStyle().Foreground(theme.Red).Bold(true).Render("due 2026-08-10")
	if !strings.Contains(view, overdueDue) {
		t.Errorf("available view = %q, want bold red overdue date", view)
	}

	entries.items = []logbook.Entry{
		{Kind: "task", ID: 7, Title: "Old chore", Status: "done", ResolvedAt: "2026-08-14T10:00:00Z"},
		{Kind: "project", ID: 8, Title: "Blog", Status: "cancelled", ResolvedAt: "2026-08-13T10:00:00Z"},
	}
	logbookView := enterRootRow(t, newModel(context.Background(), dependencies, true, time.UTC), 2).View().Content
	doneGlyph := lipgloss.NewStyle().
		Background(theme.InputBg).
		Foreground(theme.Green).
		Render("✓ ")
	cancelledGlyph := lipgloss.NewStyle().Foreground(theme.Red).Render("✗ ")
	if !strings.Contains(logbookView, doneGlyph) || !strings.Contains(logbookView, cancelledGlyph) {
		t.Errorf("logbook view = %q, want green done and red cancelled glyphs", logbookView)
	}
}

func TestAreaAndProjectContainersComposeDrillRestoreAndClamp(t *testing.T) {
	dependencies, tasks, projects, areas, _, _ := testDependencies()
	areas.items = []area.Area{{ID: 7, Title: "Home"}}
	areas.showResponses = []area.Area{
		{ID: 7, Title: "Home"},
		{ID: 7, Title: "House"},
	}
	projects.responses = [][]project.Project{
		{{ID: 11, AreaID: pointerTo(int64(7)), Title: "Kitchen reno", Status: "open"}},
		{{ID: 11, AreaID: pointerTo(int64(7)), Title: "Kitchen reno", Status: "open"}},
		{},
	}
	projects.showResponses = []project.Detail{
		{Project: project.Project{ID: 11, Title: "Kitchen reno", Status: "open"}},
		{Project: project.Project{ID: 11, Title: "Kitchen refreshed", Status: "open"}},
	}
	due := "2026-08-12"
	looseTask := task.Task{ID: 21, AreaID: pointerTo(int64(7)), Title: "Water plants", Status: "open"}
	projectTask := task.Task{
		ID: 31, ProjectID: pointerTo(int64(11)), Title: "Buy pulls", Status: "open", DueOn: &due,
	}
	tasks.listResponses = [][]task.Task{
		{looseTask},
		{projectTask},
		{projectTask},
		{looseTask},
		{projectTask},
		{},
	}

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4)
	current = enterSelection(t, current)
	view := current.View().Content
	if !slices.Equal(areas.showIDs, []int64{7}) {
		t.Fatalf("area Show IDs = %v, want 7", areas.showIDs)
	}
	if len(projects.options) != 1 || projects.options[0].Status != project.ListStatusOpen ||
		projects.options[0].AreaID == nil || *projects.options[0].AreaID != 7 {
		t.Fatalf("area project options = %#v, want open area 7", projects.options)
	}
	if len(tasks.listOptions) != 1 || tasks.listOptions[0].Status != task.ListStatusOpen ||
		tasks.listOptions[0].AreaID == nil || *tasks.listOptions[0].AreaID != 7 {
		t.Fatalf("area task options = %#v, want open area 7", tasks.listOptions)
	}
	if !containsInOrder(
		view,
		"● Home\n\n  projects\n    ◆ Kitchen reno",
		"\n\n  tasks\n    • Water plants",
	) {
		t.Fatalf("area view = %q, want spaced header then indented projects and loose tasks", view)
	}

	detail, command := press(t, current, "enter")
	if command == nil || detail.top().key != (viewKey{kind: viewAreaDetail, id: 7}) {
		t.Fatalf("area header Enter key/command = %#v/%v, want area detail", detail.top().key, command)
	}
	current, _ = press(t, current, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Kitchen reno") {
		t.Fatalf("selection after area header = %q, want first project", selected)
	}
	current = enterSelection(t, current)
	if current.top().key != (viewKey{kind: viewProject, id: 11}) {
		t.Fatalf("project key = %#v, want project 11", current.top().key)
	}
	if !slices.Equal(projects.showIDs, []int64{11}) {
		t.Fatalf("project Show IDs = %v, want 11", projects.showIDs)
	}
	if len(tasks.listOptions) != 2 || tasks.listOptions[1].ProjectID == nil ||
		*tasks.listOptions[1].ProjectID != 11 || tasks.listOptions[1].Status != task.ListStatusOpen {
		t.Fatalf("project task options = %#v, want open project 11", tasks.listOptions)
	}
	if view := current.View().Content; !containsInOrder(view, "◆ Kitchen reno\n\n  • Buy pulls", "due 2026-08-12") {
		t.Errorf("project view = %q, want spaced header then unindented task list", view)
	}

	current, _ = press(t, current, "j")
	detail, command = press(t, current, "enter")
	if command == nil || detail.top().key != (viewKey{kind: viewTaskDetail, id: 31}) {
		t.Errorf("task Enter key/command = %#v/%v, want task detail", detail.top().key, command)
	}
	current = deliver(t, detail, command)
	current = popAndReload(t, current)
	current = popAndReload(t, current)
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Kitchen reno") {
		t.Fatalf("area selection after project pop = %q, want restored project", selected)
	}
	if !strings.Contains(current.View().Content, "● House") || !slices.Equal(areas.showIDs, []int64{7, 7}) {
		t.Fatalf("refreshed area view/IDs = %q/%v, want renamed header by stable ID", current.View().Content, areas.showIDs)
	}

	current = enterSelection(t, current)
	if !strings.Contains(current.View().Content, "◆ Kitchen refreshed") ||
		!slices.Equal(projects.showIDs, []int64{11, 11, 11}) {
		t.Fatalf("refreshed project view/IDs = %q/%v, want renamed header by stable ID", current.View().Content, projects.showIDs)
	}
	current = popAndReload(t, current)
	if current.top().cursor != 0 || !strings.Contains(selectedLine(current.View().Content), "● House") {
		t.Fatalf("area cursor/view after project removal = %d/%q, want clamped refreshed header", current.top().cursor, current.View().Content)
	}
}

func TestBoardContainerUsesShowGroupingProgressAndDrillsToProject(t *testing.T) {
	dependencies, _, projects, _, boards, _ := testDependencies()
	boards.items = []board.ListedBoard{{
		Board: board.Board{ID: 4, Title: "software"},
		Stages: []board.Stage{
			{ID: 41, BoardID: 4, Title: "research"},
			{ID: 42, BoardID: 4, Title: "doing"},
			{ID: 43, BoardID: 4, Title: "review"},
		},
	}}
	stageID := int64(42)
	shown := board.Show{
		Board: board.Board{ID: 4, Title: "software"},
		Stages: []board.ShownStage{
			{Stage: board.Stage{ID: 41, BoardID: 4, Title: "research"}, Projects: []board.ShownProject{}},
			{Stage: board.Stage{ID: 42, BoardID: 4, Title: "doing"}, Projects: []board.ShownProject{
				{Project: project.Project{ID: 12, StageID: &stageID, Title: "Milestone 12", Status: "open"}, Progress: board.ProjectProgress{Done: 5, Total: 8}},
				{Project: project.Project{ID: 14, StageID: &stageID, Title: "Homelab", Status: "open"}, Progress: board.ProjectProgress{Done: 2, Total: 6}},
			}},
			{Stage: board.Stage{ID: 43, BoardID: 4, Title: "review"}, Projects: []board.ShownProject{}},
		},
	}
	renamed := shown
	renamed.Board.Title = "platform"
	boards.showResponses = []board.Show{shown, renamed}
	projects.showResponses = []project.Detail{{
		Project: project.Project{ID: 12, StageID: &stageID, Title: "Milestone 12", Status: "open"},
	}}

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 3)
	current = enterSelection(t, current)
	if boards.showCalls != 1 || !slices.Equal(boards.showIDs, []int64{4}) {
		t.Fatalf("board ShowByID calls/IDs = %d/%v, want board 4 once", boards.showCalls, boards.showIDs)
	}
	view := current.View().Content
	if !containsInOrder(view, "▥ software", "research", "doing", "Milestone 12", "5/8", "Homelab", "2/6", "review") {
		t.Fatalf("board view = %q, want glyph header then service stage/project order with progress", view)
	}
	if strings.Contains(view, "(empty)") {
		t.Errorf("board view = %q, want bare headings for empty stages", view)
	}
	detail, command := press(t, current, "enter")
	if command == nil || detail.top().key != (viewKey{kind: viewBoardDetail, id: 4}) {
		t.Fatalf("board header Enter key/command = %#v/%v, want board detail", detail.top().key, command)
	}
	current, _ = press(t, current, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Milestone 12") {
		t.Fatalf("first board project selection = %q, want first shown project", selected)
	}
	current = enterSelection(t, current)
	if current.top().key != (viewKey{kind: viewProject, id: 12}) {
		t.Errorf("board drill key = %#v, want project 12", current.top().key)
	}
	current = popAndReload(t, current)
	if !strings.Contains(current.View().Content, "platform") ||
		strings.Contains(current.View().Content, "software") ||
		!slices.Equal(boards.showIDs, []int64{4, 4}) {
		t.Errorf("refreshed board view/IDs = %q/%v, want renamed board by stable ID", current.View().Content, boards.showIDs)
	}
}

func TestNoAreaContainsExactlyLooseProjectsAcrossBoardMembership(t *testing.T) {
	dependencies, _, projects, _, _, _ := testDependencies()
	areaID := int64(7)
	stageID := int64(42)
	projects.responses = [][]project.Project{{
		{ID: 1, AreaID: &areaID, Title: "Area project", Status: "open"},
		{ID: 2, Title: "Loose unboarded", Status: "open"},
		{ID: 3, StageID: &stageID, Title: "Loose boarded", Status: "open"},
	}}

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4)
	current = enterSelection(t, current)
	if len(projects.options) != 1 || projects.options[0].Status != project.ListStatusOpen ||
		projects.options[0].AreaID != nil {
		t.Fatalf("no-area project options = %#v, want all open for client filtering", projects.options)
	}
	view := current.View().Content
	if strings.Contains(view, "Area project") || !containsInOrder(view, "(no area)", "Loose unboarded", "Loose boarded") {
		t.Fatalf("no-area view = %q, want exactly loose projects in service order", view)
	}
	if strings.Count(view, "(no area)") != 1 {
		t.Errorf("no-area view = %q, want (no area) only in the breadcrumb", view)
	}
	if selected := selectedLine(view); !strings.Contains(selected, "Loose unboarded") {
		t.Errorf("no-area first selection = %q, want first loose project", selected)
	}
	current = enterSelection(t, current)
	if current.top().key != (viewKey{kind: viewProject, id: 2}) {
		t.Errorf("no-area drill key = %#v, want project 2", current.top().key)
	}
}

func TestRowsEllipsizeFlexibleContentBeforeTrailingMetadata(t *testing.T) {
	dependencies, tasks, _, _, _, _ := testDependencies()
	due := "2026-08-12"
	tasks.inboxResponses = [][]task.ViewTask{{{
		Task: task.Task{ID: 12, Title: strings.Repeat("renovation ", 8), DueOn: &due},
	}}}
	current := newModel(context.Background(), dependencies, false, time.UTC)
	updated, _ := current.Update(tea.WindowSizeMsg{Width: 34, Height: 12})
	current = enterRootRow(t, updated.(model), 0)
	for line := range strings.SplitSeq(strings.TrimSuffix(current.View().Content, "\n"), "\n") {
		if width := lipgloss.Width(line); width > 34 {
			t.Errorf("rendered line width = %d, want at most 34: %q", width, line)
		}
	}
	selected := selectedLine(current.View().Content)
	if !strings.Contains(selected, "…") || !strings.Contains(selected, "due 2026-08-12") {
		t.Errorf("width-aware row = %q, want ellipsized title with due metadata retained", selected)
	}
}

func TestVerticalViewportKeepsSelectedRowsVisibleAfterMovementAndResize(t *testing.T) {
	assertVisible := func(name string, current model, height int, want string) {
		t.Helper()
		lines := strings.Split(strings.TrimSuffix(current.View().Content, "\n"), "\n")
		if len(lines) > height {
			t.Errorf("%s rendered lines = %d, want at most %d: %q", name, len(lines), height, current.View().Content)
		}
		if selected := selectedLine(current.View().Content); !strings.Contains(selected, want) {
			t.Errorf("%s selected line = %q, want %q visible", name, selected, want)
		}
	}

	dependencies, tasks, projects, areas, _, _ := testDependencies()
	root := newModel(context.Background(), dependencies, false, time.UTC)
	updated, _ := root.Update(tea.WindowSizeMsg{Width: 80, Height: 2})
	root = pressTimes(t, updated.(model), "j", 4)
	assertVisible("root", root, 2, "Areas")

	areaID := int64(7)
	areas.items = []area.Area{{ID: areaID, Title: "Home"}}
	areas.showResponses = []area.Area{{ID: areaID, Title: "Home"}}
	projects.responses = [][]project.Project{{
		{ID: 11, AreaID: &areaID, Title: "Project 1", Status: "open"},
		{ID: 12, AreaID: &areaID, Title: "Project 2", Status: "open"},
		{ID: 13, AreaID: &areaID, Title: "Project 3", Status: "open"},
	}}
	tasks.listResponses = [][]task.Task{{
		{ID: 21, AreaID: &areaID, Title: "Task 1", Status: "open"},
		{ID: 22, AreaID: &areaID, Title: "Task 2", Status: "open"},
		{ID: 23, AreaID: &areaID, Title: "Task 3", Status: "open"},
		{ID: 24, AreaID: &areaID, Title: "Task 4", Status: "open"},
	}}
	current := newModel(context.Background(), dependencies, false, time.UTC)
	updated, _ = current.Update(tea.WindowSizeMsg{Width: 80, Height: 5})
	current = enterRootRow(t, updated.(model), 4)
	current = enterSelection(t, current)
	current = pressTimes(t, current, "j", 7)
	assertVisible("container", current, 5, "Task 4")

	updated, _ = current.Update(tea.WindowSizeMsg{Width: 80, Height: 2})
	current = updated.(model)
	assertVisible("resized container", current, 2, "Task 4")
	current = pressTimes(t, current, "k", 7)
	assertVisible("container scrolled up", current, 2, "Home")
}

func TestDetailFieldOrderValuesCollapseAndEscaping(t *testing.T) {
	projectID := int64(3)
	due := "2026-08-15"
	deferUntil := "2026-08-12"
	deferStage := "Review"
	doneAt := "2026-08-16T10:00:00Z"
	cancelledAt := "2026-08-17T10:00:00Z"

	taskView := renderTestDetail(taskDetail(task.Task{
		ID:              9,
		ProjectID:       &projectID,
		Title:           "Ship\x1b",
		Note:            "first\x1bline\nsecond\tline",
		DueOn:           &due,
		DeferUntil:      &deferUntil,
		DeferStageTitle: &deferStage,
		Promotes:        true,
		DoneAt:          &doneAt,
		CancelledAt:     &cancelledAt,
		Status:          "done",
		Position:        4,
		CreatedAt:       "2026-08-01T10:00:00Z",
		UpdatedAt:       "2026-08-09T10:00:00Z",
		Tags:            []string{"work", "next"},
	}))
	if !containsInOrder(
		taskView,
		"✓ Ship\\x1b ↑",
		"id", "project", "note", "due on", "defer until", "defer stage", "promotes",
		"done at", "cancelled at", "status", "position", "created at", "updated at", "tags",
	) {
		t.Fatalf("task detail field order = %q", taskView)
	}
	for _, fragment := range []string{"first\\x1bline", "second\\tline", "#work #next"} {
		if !strings.Contains(taskView, fragment) {
			t.Errorf("task detail = %q, want %q", taskView, fragment)
		}
	}
	if strings.Contains(taskView, "    area") {
		t.Errorf("task detail = %q, want empty area collapsed", taskView)
	}

	projectView := renderTestDetail(projectDetail(project.Detail{
		Project: project.Project{
			ID: 7, Title: "Milestone", Note: "plan", Status: "open", Position: 2,
			CreatedAt: "created", UpdatedAt: "updated", Tags: []string{"software"},
		},
		Location: &project.Location{BoardTitle: "Soft\x1bware", StageTitle: "Doing\nNow"},
	}))
	if !containsInOrder(projectView, "◆ Milestone", "id", "board", "note", "status", "position", "created at", "updated at", "tags") ||
		!strings.Contains(projectView, "Soft\\x1bware/Doing\\nNow") {
		t.Fatalf("project detail = %q, want ordered escaped board location", projectView)
	}
	for _, collapsed := range []string{"    area", "    done at", "    cancelled at"} {
		if strings.Contains(projectView, collapsed) {
			t.Errorf("project detail = %q, want %q collapsed", projectView, collapsed)
		}
	}

	archivedAt := "2026-08-08T10:00:00Z"
	areaView := renderTestDetail(areaDetail(area.Area{
		ID: 4, Title: "Home", Note: "house", ArchivedAt: &archivedAt, Position: 1,
		CreatedAt: "created", UpdatedAt: "updated", Tags: []string{"personal"},
	}))
	if !containsInOrder(areaView, "✗ Home", "id", "note", "archived at", "position", "created at", "updated at", "tags") {
		t.Fatalf("area detail field order = %q", areaView)
	}

	boardView := renderTestDetail(boardDetail(board.Show{
		Board: board.Board{ID: 2, Title: "Software", Note: "delivery", Position: 3, CreatedAt: "created", UpdatedAt: "updated"},
		Stages: []board.ShownStage{
			{Stage: board.Stage{Title: "Research"}},
			{Stage: board.Stage{Title: "Doing"}},
			{Stage: board.Stage{Title: "Review"}},
		},
	}))
	if !containsInOrder(boardView, "▥ Software", "id", "note", "position", "stages", "Research → Doing → Review", "created at", "updated at") {
		t.Fatalf("board detail field order = %q", boardView)
	}
	if strings.Contains(boardView, "tags") {
		t.Errorf("board detail = %q, want no tags field", boardView)
	}
}

func TestDetailLoadersReadEachEntityByStableID(t *testing.T) {
	dependencies, tasks, projects, areas, boards, _ := testDependencies()
	tasks.showResponses = []task.Task{{ID: 1, Title: "task"}}
	projects.showResponses = []project.Detail{{Project: project.Project{ID: 2, Title: "project"}}}
	areas.showResponses = []area.Area{{ID: 3, Title: "area"}}
	boards.showResponses = []board.Show{{Board: board.Board{ID: 4, Title: "board"}}}
	current := newModel(context.Background(), dependencies, false, time.UTC)

	for _, key := range []viewKey{
		{kind: viewTaskDetail, id: 1},
		{kind: viewProjectDetail, id: 2},
		{kind: viewAreaDetail, id: 3},
		{kind: viewBoardDetail, id: 4},
	} {
		loaded, err := current.loadView(key)
		if err != nil || loaded.detail == nil {
			t.Fatalf("load detail %#v = %#v, %v", key, loaded, err)
		}
	}
	if !slices.Equal(tasks.showIDs, []int64{1}) ||
		!slices.Equal(projects.showIDs, []int64{2}) ||
		!slices.Equal(areas.showIDs, []int64{3}) ||
		!slices.Equal(boards.showIDs, []int64{4}) {
		t.Errorf("detail IDs = task %v, project %v, area %v, board %v", tasks.showIDs, projects.showIDs, areas.showIDs, boards.showIDs)
	}
}

func TestDetailRoutingFreshnessAndNotFoundNavigation(t *testing.T) {
	dependencies, tasks, projects, _, _, entries := testDependencies()
	tasks.inboxResponses = [][]task.ViewTask{
		{{Task: task.Task{ID: 5, Title: "task"}}},
		{{Task: task.Task{ID: 5, Title: "task"}}},
	}
	tasks.showResponses = []task.Task{
		{ID: 5, Title: "task", Note: "old", Status: "open"},
		{ID: 5, Title: "task", Note: "new", Status: "open"},
		{ID: 6, Title: "task", Status: "done"},
	}

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 0)
	current, load := press(t, current, "l")
	if current.top().key != (viewKey{kind: viewTaskDetail, id: 5}) {
		t.Fatalf("task l destination = %#v, want task detail 5", current.top().key)
	}
	current = deliver(t, current, load)
	if !strings.Contains(current.View().Content, "old") || selectedLine(current.View().Content) != "" {
		t.Fatalf("first task detail = %q, want old note and no cursor", current.View().Content)
	}
	current = popAndReload(t, current)
	current, load = press(t, current, "enter")
	current = deliver(t, current, load)
	if !strings.Contains(current.View().Content, "new") || !slices.Equal(tasks.showIDs, []int64{5, 5}) || tasks.inboxCalls != 2 {
		t.Fatalf("re-entered task detail/calls = %q/%v/%d, want fresh note and parent reload", current.View().Content, tasks.showIDs, tasks.inboxCalls)
	}

	entries.items = []logbook.Entry{
		{Kind: "project", ID: 8, Title: "project", Status: "done", ResolvedAt: "2026-08-08T11:00:00Z"},
		{Kind: "task", ID: 6, Title: "task", Status: "done", ResolvedAt: "2026-08-08T10:00:00Z"},
	}
	projects.showResponses = []project.Detail{{Project: project.Project{ID: 8, Title: "project", Status: "done"}}}
	logbookView := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 2)
	projectDetailView, load := press(t, logbookView, "enter")
	projectDetailView = deliver(t, projectDetailView, load)
	if projectDetailView.top().key != (viewKey{kind: viewProjectDetail, id: 8}) || !slices.Equal(projects.showIDs, []int64{8}) {
		t.Fatalf("logbook project destination/IDs = %#v/%v", projectDetailView.top().key, projects.showIDs)
	}
	logbookView = popAndReload(t, projectDetailView)
	logbookView, _ = press(t, logbookView, "j")
	taskDetailView, load := press(t, logbookView, "enter")
	taskDetailView = deliver(t, taskDetailView, load)
	if taskDetailView.top().key != (viewKey{kind: viewTaskDetail, id: 6}) || tasks.showIDs[len(tasks.showIDs)-1] != 6 {
		t.Fatalf("logbook task destination/IDs = %#v/%v", taskDetailView.top().key, tasks.showIDs)
	}

	missingDependencies, missingTasks, _, _, _, _ := testDependencies()
	missingTasks.inboxResponses = [][]task.ViewTask{{{Task: task.Task{ID: 99, Title: "gone"}}}}
	missingTasks.showErr = apperr.New(apperr.NotFound, "task 99 not found", nil)
	missing := enterRootRow(t, newModel(context.Background(), missingDependencies, false, time.UTC), 0)
	missing, load = press(t, missing, "enter")
	missing = deliver(t, missing, load)
	want := " gsd  Inbox ▸ gone\n\n! task 99 not found\n\n esc back\n"
	if got := missing.View().Content; got != want {
		t.Fatalf("missing detail = %q, want %q", got, want)
	}
	missing, load = press(t, missing, "esc")
	if missing.top().key.kind != viewInbox || load == nil {
		t.Fatalf("Esc from missing detail key/command = %#v/%v, want live parent reload", missing.top().key, load)
	}
}

func TestDetailViewportScrollsWithoutCursor(t *testing.T) {
	detail := taskDetail(task.Task{
		ID: 5, Title: "long", Note: "one\ntwo\nthree\nfour\nfive", Status: "open",
		Position: 1, CreatedAt: "created", UpdatedAt: "updated",
	})
	current := newModel(context.Background(), Dependencies{}, false, time.UTC)
	current.stack = append(current.stack, frame{
		key:        viewKey{kind: viewTaskDetail, id: 5},
		loadedView: loadedView{detail: &detail},
	})
	updated, _ := current.Update(tea.WindowSizeMsg{Width: 80, Height: 3})
	current = updated.(model)
	initial := current.View().Content
	if len(strings.Split(strings.TrimSuffix(initial, "\n"), "\n")) != 3 || selectedLine(initial) != "" {
		t.Fatalf("initial detail viewport = %q, want three lines and no cursor", initial)
	}
	current, _ = press(t, current, "down")
	if current.top().offset != 1 || current.View().Content == initial {
		t.Fatalf("detail down offset/view = %d/%q, want one-line scroll", current.top().offset, current.View().Content)
	}
	current = pressTimes(t, current, "j", 99)
	lines, _ := current.renderLines(current.top())
	if current.top().offset != len(lines)-1 {
		t.Errorf("detail bottom offset = %d, want %d", current.top().offset, len(lines)-1)
	}
	current, _ = press(t, current, "up")
	if current.top().offset != len(lines)-2 {
		t.Errorf("detail up offset = %d, want %d", current.top().offset, len(lines)-2)
	}
}

func renderTestDetail(detail detailView) string {
	current := newModel(context.Background(), Dependencies{}, false, time.UTC)
	current.stack = append(current.stack, frame{loadedView: loadedView{detail: &detail}})
	return current.View().Content
}

func responseAt[T any](responses [][]T, call int) []T {
	if len(responses) == 0 {
		return nil
	}
	return responses[min(call, len(responses)-1)]
}

func valueAt[T any](responses []T, call int) T {
	if len(responses) == 0 {
		var zero T
		return zero
	}
	return responses[min(call, len(responses)-1)]
}

func press(t *testing.T, current model, key string) (model, tea.Cmd) {
	t.Helper()
	message := tea.KeyPressMsg{Text: key, Code: []rune(key)[0]}
	switch key {
	case "ctrl+c":
		message = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "enter":
		message = tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		message = tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		message = tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		message = tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		message = tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	updated, command := current.Update(message)
	return updated.(model), command
}

func pressTimes(t *testing.T, current model, key string, count int) model {
	t.Helper()
	for range count {
		current, _ = press(t, current, key)
	}
	return current
}

func deliver(t *testing.T, current model, command tea.Cmd) model {
	t.Helper()
	if command == nil {
		t.Fatal("load command = nil")
	}
	updated, _ := current.Update(command())
	return updated.(model)
}

func enterRootRow(t *testing.T, current model, index int) model {
	t.Helper()
	current = pressTimes(t, current, "j", index)
	current, load := press(t, current, "enter")
	return deliver(t, current, load)
}

func enterSelection(t *testing.T, current model) model {
	t.Helper()
	current, load := press(t, current, "enter")
	return deliver(t, current, load)
}

func popAndReload(t *testing.T, current model) model {
	t.Helper()
	current, load := press(t, current, "esc")
	return deliver(t, current, load)
}

func pointerTo[T any](value T) *T {
	return &value
}

func containsInOrder(content string, values ...string) bool {
	for _, value := range values {
		index := strings.Index(content, value)
		if index < 0 {
			return false
		}
		content = content[index+len(value):]
	}
	return true
}

func assertQuit(t *testing.T, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatal("quit command = nil")
	}
	result := command()
	if _, ok := result.(tea.QuitMsg); !ok {
		t.Errorf("quit command result = %T, want tea.QuitMsg", result)
	}
}

func selectedLine(view string) string {
	for line := range strings.SplitSeq(view, "\n") {
		if strings.HasPrefix(line, "▌ ") {
			return line
		}
	}
	return ""
}
