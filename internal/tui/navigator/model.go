package navigator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
)

type TaskReader interface {
	Inbox(context.Context) ([]task.ViewTask, error)
	Available(context.Context) ([]task.ViewTask, error)
	List(context.Context, task.ListOptions) ([]task.Task, error)
	Show(context.Context, int64) (task.Task, error)
}

type ProjectReader interface {
	List(context.Context, project.ListOptions) ([]project.Project, error)
	Show(context.Context, int64) (project.Detail, error)
}

type AreaReader interface {
	List(context.Context, area.ListOptions) ([]area.Area, error)
	Show(context.Context, int64) (area.Area, error)
}

type BoardReader interface {
	List(context.Context) ([]board.ListedBoard, error)
	ShowByID(context.Context, int64) (board.Show, error)
}

type LogbookReader interface {
	List(context.Context) ([]logbook.Entry, error)
}

type Dependencies struct {
	Tasks    TaskReader
	Projects ProjectReader
	Areas    AreaReader
	Boards   BoardReader
	Logbook  LogbookReader
}

func Run(
	ctx context.Context,
	dependencies Dependencies,
	options tui.ProgramOptions,
	location *time.Location,
) error {
	if location == nil {
		location = time.Local
	}
	_, err := tui.RunProgram(
		ctx,
		newModel(ctx, dependencies, options.Color != tui.ColorDisabled, location),
		options,
	)
	return err
}

type viewKind uint8

const (
	viewRoot viewKind = iota
	viewInbox
	viewAvailable
	viewLogbook
	viewBoards
	viewAreas
	viewArea
	viewProject
	viewBoard
	viewNoArea
)

type viewKey struct {
	kind viewKind
	id   int64
}

type rowStyle uint8

const (
	rowStandard rowStyle = iota
	rowAreaHeader
	rowProjectHeader
	rowBoardHeader
)

type row struct {
	identity    string
	cells       []string
	style       rowStyle
	destination viewKey
	descends    bool
}

type section struct {
	title       string
	columns     []string
	rightAlign  int
	flexColumn  int
	showColumns bool
	showEmpty   bool
	firstGap    int
	rows        []row
}

type loadedView struct {
	plainTitle string
	header     *row
	sections   []section
}

type cursorState struct {
	identity string
	index    int
}

type frame struct {
	key        viewKey
	generation uint64
	loading    bool
	err        error
	loadedView
	cursor int
}

type loadResultMsg struct {
	key        viewKey
	generation uint64
	loadedView
	err error
}

type model struct {
	ctx          context.Context
	dependencies Dependencies
	location     *time.Location
	theme        tui.Theme
	colorEnabled bool
	width        int
	generation   uint64
	stack        []frame
	cursors      map[viewKey]cursorState
}

func newModel(
	ctx context.Context,
	dependencies Dependencies,
	colorEnabled bool,
	location *time.Location,
) model {
	if location == nil {
		location = time.Local
	}
	rootKey := viewKey{kind: viewRoot}
	root := frame{
		key: rootKey,
		loadedView: loadedView{sections: []section{{
			rightAlign: -1,
			flexColumn: 0,
			rows:       rootRows(),
		}}},
	}
	return model{
		ctx:          ctx,
		dependencies: dependencies,
		location:     location,
		theme:        tui.ThemeForBackground(true),
		colorEnabled: colorEnabled,
		stack:        []frame{root},
		cursors:      map[viewKey]cursorState{},
	}
}

func (m model) Init() tea.Cmd {
	if m.colorEnabled {
		return tea.RequestBackgroundColor
	}
	return nil
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.BackgroundColorMsg:
		if m.colorEnabled {
			m.theme = tui.ThemeForBackground(message.IsDark())
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = message.Width
		return m, nil
	case loadResultMsg:
		return m.applyLoad(message), nil
	case tea.KeyPressMsg:
		return m.updateKey(message)
	default:
		return m, nil
	}
}

func (m model) updateKey(message tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if len(m.stack) == 1 {
			return m, tea.Quit
		}
		return m.pop()
	case "h":
		if len(m.stack) > 1 {
			return m.pop()
		}
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "enter", "l":
		return m.pushSelection()
	}
	return m, nil
}

func (m *model) move(delta int) {
	current := m.top()
	rows := current.selectableRows()
	if current.loading || current.err != nil || len(rows) == 0 {
		return
	}
	current.cursor = clamp(current.cursor+delta, 0, len(rows)-1)
	m.rememberCursor(current)
}

func (m model) pushSelection() (tea.Model, tea.Cmd) {
	current := m.top()
	selected, ok := current.selectedRow()
	if !ok || !selected.descends {
		return m, nil
	}
	m.rememberCursor(current)
	m.stack = append(m.stack, frame{key: selected.destination})
	return m.enterTop()
}

func (m model) pop() (tea.Model, tea.Cmd) {
	m.rememberCursor(m.top())
	m.stack = m.stack[:len(m.stack)-1]
	return m.enterTop()
}

func (m model) enterTop() (tea.Model, tea.Cmd) {
	current := m.top()
	state := m.cursors[current.key]
	if current.key.kind == viewRoot {
		current.loading = false
		current.err = nil
		current.loadedView = loadedView{sections: []section{{
			rightAlign: -1,
			flexColumn: 0,
			rows:       rootRows(),
		}}}
		current.cursor = restoreCursor(current.selectableRows(), state)
		m.rememberCursor(current)
		return m, nil
	}

	m.generation++
	current.generation = m.generation
	current.loading = true
	current.err = nil
	current.loadedView = loadedView{}
	current.cursor = state.index
	generation := current.generation
	key := current.key
	return m, m.loadCommand(key, generation)
}

func (m model) loadCommand(key viewKey, generation uint64) tea.Cmd {
	return func() tea.Msg {
		loaded, err := m.loadView(key)
		return loadResultMsg{
			key:        key,
			generation: generation,
			loadedView: loaded,
			err:        err,
		}
	}
}

func (m model) loadView(key viewKey) (loadedView, error) {
	switch key.kind {
	case viewInbox:
		return m.loadInbox()
	case viewAvailable:
		return m.loadAvailable()
	case viewLogbook:
		return m.loadLogbook()
	case viewBoards:
		return m.loadBoards()
	case viewAreas:
		return m.loadAreas()
	case viewArea:
		return m.loadArea(key.id)
	case viewProject:
		return m.loadProject(key.id)
	case viewBoard:
		return m.loadBoard(key.id)
	case viewNoArea:
		return m.loadNoArea()
	case viewRoot:
		return loadedView{}, fmt.Errorf("root view has no loader")
	default:
		return loadedView{}, fmt.Errorf("unknown view kind %d", key.kind)
	}
}

func (m model) loadInbox() (loadedView, error) {
	items, err := m.dependencies.Tasks.Inbox(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{taskViewSection(viewTaskRows(items))}}, nil
}

func (m model) loadAvailable() (loadedView, error) {
	items, err := m.dependencies.Tasks.Available(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{taskViewSection(viewTaskRows(items))}}, nil
}

func (m model) loadLogbook() (loadedView, error) {
	entries, err := m.dependencies.Logbook.List(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	rows, err := logbookRows(entries, m.location)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{{
		columns:     []string{"kind", "id", "title", "status", "date"},
		rightAlign:  1,
		flexColumn:  2,
		showColumns: true,
		rows:        rows,
	}}}, nil
}

func (m model) loadBoards() (loadedView, error) {
	items, err := m.dependencies.Boards.List(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{{
		columns:     []string{"board", "stages"},
		rightAlign:  -1,
		flexColumn:  1,
		showColumns: true,
		rows:        boardRows(items),
	}}}, nil
}

func (m model) loadAreas() (loadedView, error) {
	items, err := m.dependencies.Areas.List(m.ctx, area.ListOptions{Slice: area.ListSliceActive})
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{{
		columns:     []string{"id", "title", "state"},
		rightAlign:  0,
		flexColumn:  1,
		showColumns: true,
		rows:        areaRows(items),
	}}}, nil
}

func (m model) loadArea(id int64) (loadedView, error) {
	shown, err := m.dependencies.Areas.Show(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	projects, err := m.dependencies.Projects.List(m.ctx, project.ListOptions{
		Status: project.ListStatusOpen,
		AreaID: &id,
	})
	if err != nil {
		return loadedView{}, err
	}
	tasks, err := m.dependencies.Tasks.List(m.ctx, task.ListOptions{
		Status: task.ListStatusOpen,
		AreaID: &id,
	})
	if err != nil {
		return loadedView{}, err
	}
	header := containerHeader(rowAreaHeader, id, shown.Title)
	return loadedView{
		header: &header,
		sections: []section{
			entitySection("projects", projectRows(projects), true),
			listedTaskSection("tasks", listedTaskRows(tasks), true),
		},
	}, nil
}

func (m model) loadProject(id int64) (loadedView, error) {
	shown, err := m.dependencies.Projects.Show(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	tasks, err := m.dependencies.Tasks.List(m.ctx, task.ListOptions{
		Status:    task.ListStatusOpen,
		ProjectID: &id,
	})
	if err != nil {
		return loadedView{}, err
	}
	header := containerHeader(rowProjectHeader, id, shown.Title)
	return loadedView{
		header:   &header,
		sections: []section{listedTaskSection("", listedTaskRows(tasks), false)},
	}, nil
}

func (m model) loadBoard(id int64) (loadedView, error) {
	shown, err := m.dependencies.Boards.ShowByID(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	header := containerHeader(rowBoardHeader, id, shown.Board.Title)
	sections := make([]section, 0, len(shown.Stages))
	for _, stage := range shown.Stages {
		sections = append(sections, section{
			title:      stage.Title,
			rightAlign: 1,
			flexColumn: 2,
			showEmpty:  true,
			firstGap:   1,
			rows:       boardProjectRows(stage.Projects),
		})
	}
	return loadedView{
		header:   &header,
		sections: sections,
	}, nil
}

func (m model) loadNoArea() (loadedView, error) {
	projects, err := m.dependencies.Projects.List(m.ctx, project.ListOptions{Status: project.ListStatusOpen})
	if err != nil {
		return loadedView{}, err
	}
	loose := make([]project.Project, 0, len(projects))
	for _, current := range projects {
		if current.AreaID == nil {
			loose = append(loose, current)
		}
	}
	return loadedView{
		plainTitle: "(no area)",
		sections:   []section{entitySection("", projectRows(loose), false)},
	}, nil
}

func (m model) applyLoad(message loadResultMsg) model {
	current := m.top()
	if current.key != message.key || current.generation != message.generation {
		return m
	}
	current.loading = false
	current.err = message.err
	current.loadedView = message.loadedView
	state := m.cursors[current.key]
	current.cursor = restoreCursor(current.selectableRows(), state)
	m.rememberCursor(current)
	return m
}

func (m *model) rememberCursor(current *frame) {
	state := m.cursors[current.key]
	state.index = current.cursor
	rows := current.selectableRows()
	if current.cursor >= 0 && current.cursor < len(rows) {
		state.identity = rows[current.cursor].identity
	}
	m.cursors[current.key] = state
}

func (m *model) top() *frame {
	return &m.stack[len(m.stack)-1]
}

func (f frame) selectableRows() []row {
	count := 0
	if f.header != nil {
		count++
	}
	for _, current := range f.sections {
		count += len(current.rows)
	}
	rows := make([]row, 0, count)
	if f.header != nil {
		rows = append(rows, *f.header)
	}
	for _, current := range f.sections {
		rows = append(rows, current.rows...)
	}
	return rows
}

func (f frame) selectedRow() (row, bool) {
	rows := f.selectableRows()
	if f.cursor < 0 || f.cursor >= len(rows) {
		return row{}, false
	}
	return rows[f.cursor], true
}

func restoreCursor(rows []row, state cursorState) int {
	if len(rows) == 0 {
		return 0
	}
	if state.identity != "" {
		for index := range rows {
			if rows[index].identity == state.identity {
				return index
			}
		}
	}
	return clamp(state.index, 0, len(rows)-1)
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}

func rootRows() []row {
	return []row{
		descendingRow("root:inbox", []string{"Inbox"}, viewKey{kind: viewInbox}),
		descendingRow("root:available", []string{"Available"}, viewKey{kind: viewAvailable}),
		descendingRow("root:logbook", []string{"Logbook"}, viewKey{kind: viewLogbook}),
		descendingRow("root:boards", []string{"Boards"}, viewKey{kind: viewBoards}),
		descendingRow("root:areas", []string{"Areas"}, viewKey{kind: viewAreas}),
	}
}

func viewTaskRows(items []task.ViewTask) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		rows = append(rows, row{
			identity: "task:" + strconv.FormatInt(item.ID, 10),
			cells: []string{
				strconv.FormatInt(item.ID, 10),
				taskTitle(item.Task),
				taskDates(item.Task),
			},
		})
	}
	return rows
}

func listedTaskRows(items []task.Task) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		rows = append(rows, row{
			identity: "task:" + strconv.FormatInt(item.ID, 10),
			cells: []string{
				strconv.FormatInt(item.ID, 10),
				taskTitle(item),
				item.Status,
				taskDates(item),
			},
		})
	}
	return rows
}

func logbookRows(entries []logbook.Entry, location *time.Location) ([]row, error) {
	rows := make([]row, 0, len(entries))
	for _, entry := range entries {
		resolvedAt, err := time.Parse(time.RFC3339Nano, entry.ResolvedAt)
		if err != nil {
			return nil, fmt.Errorf("parse logbook resolved_at for %s %d: %w", entry.Kind, entry.ID, err)
		}
		rows = append(rows, row{
			identity: entry.Kind + ":" + strconv.FormatInt(entry.ID, 10),
			cells: []string{
				entry.Kind,
				strconv.FormatInt(entry.ID, 10),
				entry.Title,
				entry.Status,
				resolvedAt.In(location).Format(time.DateOnly),
			},
		})
	}
	return rows, nil
}

func boardRows(items []board.ListedBoard) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		stageTitles := make([]string, len(item.Stages))
		for index := range item.Stages {
			stageTitles[index] = item.Stages[index].Title
		}
		rows = append(rows, descendingRow(
			"board:"+strconv.FormatInt(item.ID, 10),
			[]string{item.Title, joinStages(stageTitles)},
			viewKey{kind: viewBoard, id: item.ID},
		))
	}
	return rows
}

func areaRows(items []area.Area) []row {
	rows := make([]row, 0, len(items)+1)
	for _, item := range items {
		state := ""
		if item.ArchivedAt != nil {
			state = "archived"
		}
		rows = append(rows, descendingRow(
			"area:"+strconv.FormatInt(item.ID, 10),
			[]string{strconv.FormatInt(item.ID, 10), item.Title, state},
			viewKey{kind: viewArea, id: item.ID},
		))
	}
	return append(rows, descendingRow(
		"area:none",
		[]string{"", "(no area)", ""},
		viewKey{kind: viewNoArea},
	))
}

func projectRows(items []project.Project) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		rows = append(rows, descendingRow(
			"project:"+strconv.FormatInt(item.ID, 10),
			[]string{strconv.FormatInt(item.ID, 10), item.Title, item.Status},
			viewKey{kind: viewProject, id: item.ID},
		))
	}
	return rows
}

func boardProjectRows(items []board.ShownProject) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		rows = append(rows, descendingRow(
			"project:"+strconv.FormatInt(item.ID, 10),
			[]string{
				"◆",
				strconv.FormatInt(item.ID, 10),
				item.Title,
				fmt.Sprintf("%d/%d", item.Progress.Done, item.Progress.Total),
			},
			viewKey{kind: viewProject, id: item.ID},
		))
	}
	return rows
}

func descendingRow(identity string, cells []string, destination viewKey) row {
	return row{identity: identity, cells: cells, destination: destination, descends: true}
}

func containerHeader(style rowStyle, id int64, title string) row {
	return row{
		identity: "header:" + strconv.FormatInt(id, 10),
		cells:    []string{strconv.FormatInt(id, 10), title},
		style:    style,
	}
}

func taskViewSection(rows []row) section {
	return section{
		columns:     []string{"id", "title", "dates"},
		rightAlign:  0,
		flexColumn:  1,
		showColumns: true,
		rows:        rows,
	}
}

func listedTaskSection(title string, rows []row, showEmpty bool) section {
	return section{
		title:       title,
		columns:     []string{"id", "title", "status", "dates"},
		rightAlign:  0,
		flexColumn:  1,
		showColumns: true,
		showEmpty:   showEmpty,
		rows:        rows,
	}
}

func entitySection(title string, rows []row, showEmpty bool) section {
	return section{
		title:       title,
		columns:     []string{"id", "title", "status"},
		rightAlign:  0,
		flexColumn:  1,
		showColumns: true,
		showEmpty:   showEmpty,
		rows:        rows,
	}
}
