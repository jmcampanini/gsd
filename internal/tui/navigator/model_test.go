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
	inboxErr           error
	availableErr       error
	listErr            error
	inboxCalls         int
	availableCalls     int
	listCalls          int
	listOptions        []task.ListOptions
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

func (*fakeTasks) Show(context.Context, int64) (task.Task, error) {
	return task.Task{}, nil
}

type fakeProjects struct {
	responses [][]project.Project
	err       error
	calls     int
	options   []project.ListOptions
}

func (f *fakeProjects) List(_ context.Context, options project.ListOptions) ([]project.Project, error) {
	response := responseAt(f.responses, f.calls)
	f.calls++
	f.options = append(f.options, options)
	return response, f.err
}

func (*fakeProjects) Show(context.Context, int64) (project.Detail, error) {
	return project.Detail{}, nil
}

type fakeAreas struct {
	items   []area.Area
	err     error
	calls   int
	options []area.ListOptions
}

func (f *fakeAreas) List(_ context.Context, options area.ListOptions) ([]area.Area, error) {
	f.calls++
	f.options = append(f.options, options)
	return f.items, f.err
}

func (*fakeAreas) Show(context.Context, int64) (area.Area, error) {
	return area.Area{}, nil
}

type fakeBoards struct {
	items      []board.ListedBoard
	showResult board.Show
	err        error
	showErr    error
	calls      int
	showCalls  int
	showTitles []string
}

func (f *fakeBoards) List(context.Context) ([]board.ListedBoard, error) {
	f.calls++
	return f.items, f.err
}

func (f *fakeBoards) Show(_ context.Context, title string) (board.Show, error) {
	f.showCalls++
	f.showTitles = append(f.showTitles, title)
	return f.showResult, f.showErr
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
	wantRoot := "> Inbox\n  Available\n  Logbook\n  Boards\n  Areas\n"
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
		if got := updated.View().Content; got != "  loading…\n" {
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
	projects.responses = [][]project.Project{{{
		ID: 11, AreaID: &areaID, Title: "Kitchen reno", Status: "open",
	}}}

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
	if got := current.View().Content; got != wantError+"\n" {
		t.Errorf("error view = %q, want %q", got, wantError+"\n")
	}
	current, command := press(t, current, "esc")
	if command != nil || len(current.stack) != 1 {
		t.Errorf("Esc from failed view stack/command = %d/%v, want live root", len(current.stack), command)
	}
}

func TestTaskAndLogbookRowsMirrorCLIColumns(t *testing.T) {
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
		for _, fragment := range []string{
			"id  title",
			`12  Ship\x1b ↑`,
			`due 2026-08-08 defer 2026-08-09 defer→Review\tNow`,
		} {
			if !strings.Contains(view, fragment) {
				t.Errorf("task view %q missing %q", view, fragment)
			}
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
	for _, fragment := range []string{
		"kind  id  title       status  date",
		"task   7  Done thing  done    2026-08-07",
	} {
		if !strings.Contains(view, fragment) {
			t.Errorf("logbook view %q missing %q", view, fragment)
		}
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
	for _, fragment := range []string{"board   stages", "Second  Research → Doing → Review"} {
		if !strings.Contains(boardView, fragment) {
			t.Errorf("board view %q missing %q", boardView, fragment)
		}
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
	for _, fragment := range []string{"id  title      state", "4  Home", "8  Work", "(no area)"} {
		if !strings.Contains(areaView, fragment) {
			t.Errorf("area view %q missing %q", areaView, fragment)
		}
	}
}

func TestEmptyListsMatchCLIAndAreasRetainsPseudoRow(t *testing.T) {
	dependencies, _, _, _, _, _ := testDependencies()
	for _, rootIndex := range []int{0, 1, 2, 3} {
		current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), rootIndex)
		if got := current.View().Content; got != "" {
			t.Errorf("empty root row %d view = %q, want no table", rootIndex, got)
		}
	}
	areas := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4)
	if got := areas.View().Content; !strings.Contains(got, "id  title      state") ||
		!strings.Contains(got, ">     (no area)") {
		t.Errorf("empty areas view = %q, want header and selected pseudo-row", got)
	}
}

func TestSelectedMarkerUsesAccentWithoutStylingWholeRowAndBackgroundCanChange(t *testing.T) {
	dependencies, _, _, _, _, _ := testDependencies()
	current := newModel(context.Background(), dependencies, true, time.UTC)
	if current.Init() == nil {
		t.Fatal("colored Init command = nil, want background detection")
	}
	darkTheme := tui.ThemeForBackground(true)
	darkMarker := lipgloss.NewStyle().Foreground(darkTheme.Accent).Render("> ")
	if got := current.View().Content; !strings.HasPrefix(got, darkMarker+"Inbox") {
		t.Errorf("dark selected row = %q, want accent marker then plain label", got)
	}
	updated, _ := current.Update(tea.BackgroundColorMsg{Color: lipgloss.Color("#ffffff")})
	current = updated.(model)
	lightMarker := lipgloss.NewStyle().Foreground(tui.ThemeForBackground(false).Accent).Render("> ")
	if got := current.View().Content; !strings.HasPrefix(got, lightMarker+"Inbox") {
		t.Errorf("light selected row = %q, want detected accent marker", got)
	}
	current, _ = press(t, current, "j")
	lines := strings.Split(current.View().Content, "\n")
	if lines[0] != "  Inbox" || !strings.HasPrefix(lines[1], lightMarker+"Available") {
		t.Errorf("moved marker lines = %#v, want persistent marker only on selection", lines)
	}

	loading, _ := press(t, newModel(context.Background(), dependencies, true, time.UTC), "enter")
	dimLoading := lipgloss.NewStyle().Foreground(darkTheme.Dim).Render("loading…")
	if got, want := loading.View().Content, "  "+dimLoading+"\n"; got != want {
		t.Errorf("colored loading view = %q, want %q", got, want)
	}
}

func TestAreaAndProjectContainersComposeDrillRestoreAndClamp(t *testing.T) {
	dependencies, tasks, projects, areas, _, _ := testDependencies()
	areas.items = []area.Area{{ID: 7, Title: "Home"}}
	projects.responses = [][]project.Project{
		{{ID: 11, AreaID: pointerTo(int64(7)), Title: "Kitchen reno", Status: "open"}},
		{{ID: 11, AreaID: pointerTo(int64(7)), Title: "Kitchen reno", Status: "open"}},
		{},
	}
	due := "2026-08-12"
	looseTask := task.Task{ID: 21, AreaID: pointerTo(int64(7)), Title: "Water plants", Status: "open"}
	projectTask := task.Task{
		ID: 31, ProjectID: pointerTo(int64(11)), Title: "Buy pulls", Status: "open", DueOn: &due,
	}
	tasks.listResponses = [][]task.Task{
		{looseTask},
		{projectTask},
		{looseTask},
		{projectTask},
		{},
	}

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 4)
	current = enterSelection(t, current)
	view := current.View().Content
	if len(projects.options) != 1 || projects.options[0].Status != project.ListStatusOpen ||
		projects.options[0].AreaID == nil || *projects.options[0].AreaID != 7 {
		t.Fatalf("area project options = %#v, want open area 7", projects.options)
	}
	if len(tasks.listOptions) != 1 || tasks.listOptions[0].Status != task.ListStatusOpen ||
		tasks.listOptions[0].AreaID == nil || *tasks.listOptions[0].AreaID != 7 {
		t.Fatalf("area task options = %#v, want open area 7", tasks.listOptions)
	}
	if !containsInOrder(view, "● 7  Home", "projects", "Kitchen reno", "tasks", "Water plants") {
		t.Fatalf("area view = %q, want header, projects, then loose tasks", view)
	}
	if !strings.Contains(view, "id  title") || !strings.Contains(view, "status") {
		t.Errorf("area view = %q, want CLI project/task columns", view)
	}

	unchanged, command := press(t, current, "enter")
	if command != nil || len(unchanged.stack) != 3 {
		t.Fatalf("area header Enter stack/command = %d/%v, want inert until detail chunk", len(unchanged.stack), command)
	}
	current, _ = press(t, unchanged, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Kitchen reno") {
		t.Fatalf("selection after area header = %q, want first project", selected)
	}
	current = enterSelection(t, current)
	if current.top().key != (viewKey{kind: viewProject, id: 11}) {
		t.Fatalf("project key = %#v, want project 11", current.top().key)
	}
	if len(tasks.listOptions) != 2 || tasks.listOptions[1].ProjectID == nil ||
		*tasks.listOptions[1].ProjectID != 11 || tasks.listOptions[1].Status != task.ListStatusOpen {
		t.Fatalf("project task options = %#v, want open project 11", tasks.listOptions)
	}
	if view := current.View().Content; !containsInOrder(view, "◆ 11  Kitchen reno", "Buy pulls", "open", "due 2026-08-12") {
		t.Errorf("project view = %q, want header then ordered task list", view)
	}

	current, _ = press(t, current, "j")
	unchanged, command = press(t, current, "enter")
	if command != nil || len(unchanged.stack) != 4 {
		t.Errorf("task Enter stack/command = %d/%v, want inert until detail chunk", len(unchanged.stack), command)
	}
	current = popAndReload(t, unchanged)
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Kitchen reno") {
		t.Fatalf("area selection after project pop = %q, want restored project", selected)
	}

	current = enterSelection(t, current)
	current = popAndReload(t, current)
	if current.top().cursor != 0 || !strings.Contains(selectedLine(current.View().Content), "● 7  Home") {
		t.Fatalf("area cursor/view after project removal = %d/%q, want clamped header", current.top().cursor, current.View().Content)
	}
}

func TestBoardContainerUsesShowGroupingProgressAndDrillsToProject(t *testing.T) {
	dependencies, _, _, _, boards, _ := testDependencies()
	boards.items = []board.ListedBoard{{
		Board: board.Board{ID: 4, Title: "software"},
		Stages: []board.Stage{
			{ID: 41, BoardID: 4, Title: "research"},
			{ID: 42, BoardID: 4, Title: "doing"},
			{ID: 43, BoardID: 4, Title: "review"},
		},
	}}
	stageID := int64(42)
	boards.showResult = board.Show{
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

	current := enterRootRow(t, newModel(context.Background(), dependencies, false, time.UTC), 3)
	current = enterSelection(t, current)
	if boards.showCalls != 1 || !slices.Equal(boards.showTitles, []string{"software"}) {
		t.Fatalf("board Show calls/titles = %d/%v, want software once", boards.showCalls, boards.showTitles)
	}
	view := current.View().Content
	if !containsInOrder(view, "software", "research", "(empty)", "doing", "Milestone 12", "5/8", "Homelab", "2/6", "review", "(empty)") {
		t.Fatalf("board view = %q, want service stage/project order with progress", view)
	}
	unchanged, command := press(t, current, "enter")
	if command != nil || len(unchanged.stack) != 3 {
		t.Fatalf("board header Enter stack/command = %d/%v, want inert until detail chunk", len(unchanged.stack), command)
	}
	current, _ = press(t, unchanged, "j")
	if selected := selectedLine(current.View().Content); !strings.Contains(selected, "Milestone 12") {
		t.Fatalf("first board project selection = %q, want first shown project", selected)
	}
	current = enterSelection(t, current)
	if current.top().key != (viewKey{kind: viewProject, id: 12}) {
		t.Errorf("board drill key = %#v, want project 12", current.top().key)
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
	if strings.HasPrefix(strings.Split(view, "\n")[0], "> ") {
		t.Errorf("no-area title = %q, want plain non-selectable title", strings.Split(view, "\n")[0])
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

func responseAt[T any](responses [][]T, call int) []T {
	if len(responses) == 0 {
		return nil
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
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Errorf("quit command result = %T, want tea.QuitMsg", command())
	}
}

func selectedLine(view string) string {
	for line := range strings.SplitSeq(view, "\n") {
		if strings.HasPrefix(line, "> ") {
			return line
		}
	}
	return ""
}
