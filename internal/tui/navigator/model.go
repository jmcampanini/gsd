package navigator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"charm.land/bubbles/v2/textinput"
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
	viewTaskDetail
	viewProjectDetail
	viewAreaDetail
	viewBoardDetail
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

type cellAccent uint8

const (
	accentPlain cellAccent = iota
	accentDim
	accentGreen
	accentRed
	accentYellow
	accentOverdue
)

type row struct {
	identity       string
	crumb          string
	cells          []string
	accents        []cellAccent
	matchPositions [][]int
	style          rowStyle
	destination    viewKey
}

type section struct {
	title      string
	flexColumn int
	firstGap   int
	hideEmpty  bool
	rows       []row
}

type detailKind uint8

const (
	detailTask detailKind = iota
	detailProject
	detailArea
	detailBoard
)

type detailField struct {
	label             string
	value             string
	preserveLineFeeds bool
}

type detailView struct {
	kind     detailKind
	id       int64
	title    string
	status   string
	promotes bool
	fields   []detailField
}

type loadedView struct {
	header   *row
	sections []section
	detail   *detailView
}

type cursorState struct {
	identity string
	index    int
}

type filterState struct {
	enabled      bool
	editing      bool
	input        textinput.Model
	origin       cursorState
	originOffset int
}

type frame struct {
	key        viewKey
	crumb      string
	generation uint64
	loading    bool
	err        error
	loadedView
	filter filterState
	cursor int
	offset int
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
	now          func() time.Time
	theme        tui.Theme
	colorEnabled bool
	width        int
	height       int
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
	root := frame{key: rootKey, loadedView: rootView()}
	return model{
		ctx:          ctx,
		dependencies: dependencies,
		location:     location,
		now:          time.Now,
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
		m.height = message.Height
		m.resizeFilterInput(m.top())
		m.ensureCursorVisible(m.top())
		return m, nil
	case loadResultMsg:
		return m.applyLoad(message), nil
	case tea.KeyPressMsg:
		return m.updateKey(message)
	case tea.PasteMsg:
		current := m.top()
		if !current.filter.editing {
			return m, nil
		}
		selected, _ := current.selectedRow()
		previousIndex := current.cursor
		var command tea.Cmd
		current.filter.input, command = current.filter.input.Update(message)
		m.filterChanged(current, selected.identity, previousIndex)
		return m, command
	default:
		return m, nil
	}
}

func (m model) updateKey(message tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	current := m.top()
	key := message.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if current.filter.editing {
		switch key {
		case "esc":
			m.clearFilter(current)
			return m, nil
		case "/":
			if current.filter.input.Value() == "" {
				m.clearFilter(current)
			} else {
				current.filter.editing = false
				current.filter.input.Blur()
			}
			return m, nil
		case "enter":
			return m.pushSelection()
		}
		before := current.filter.input.Value()
		selected, _ := current.selectedRow()
		previousIndex := current.cursor
		var command tea.Cmd
		current.filter.input, command = current.filter.input.Update(message)
		if current.filter.input.Value() != before {
			m.filterChanged(current, selected.identity, previousIndex)
		}
		return m, command
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "/":
		if m.canFilter(current) {
			m.startFilter(current)
		}
	case "esc":
		if current.filter.enabled {
			m.clearFilter(current)
			return m, nil
		}
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
	if current.loading || current.err != nil {
		return
	}
	if current.detail != nil {
		lines, _ := m.renderLines(current)
		maximum := 0
		if budget := m.contentHeight(); budget > 0 {
			maximum = max(len(lines)-budget, 0)
		}
		current.offset = clamp(current.offset+delta, 0, maximum)
		return
	}
	rows := current.selectableRows()
	if len(rows) == 0 {
		return
	}
	current.cursor = clamp(current.cursor+delta, 0, len(rows)-1)
	m.ensureCursorVisible(current)
	if !current.filter.enabled {
		m.rememberCursor(current)
	}
}

func (m model) pushSelection() (tea.Model, tea.Cmd) {
	current := m.top()
	selected, ok := current.selectedRow()
	if !ok {
		return m, nil
	}
	if current.filter.enabled {
		current.filter = filterState{}
		current.cursor = clampCursor(rowIndex(current.selectableRows(), selected.identity), len(current.selectableRows()))
		m.ensureCursorVisible(current)
	}
	m.rememberCursor(current)
	m.stack = append(m.stack, frame{key: selected.destination, crumb: selected.crumb})
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
		current.loadedView = rootView()
		current.cursor = restoreCursor(current.selectableRows(), state)
		m.ensureCursorVisible(current)
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
	case viewTaskDetail:
		return m.loadTaskDetail(key.id)
	case viewProjectDetail:
		return m.loadProjectDetail(key.id)
	case viewAreaDetail:
		return m.loadAreaDetail(key.id)
	case viewBoardDetail:
		return m.loadBoardDetail(key.id)
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
	return loadedView{sections: []section{taskSection("", viewTaskTasks(items), m.today())}}, nil
}

func (m model) loadAvailable() (loadedView, error) {
	items, err := m.dependencies.Tasks.Available(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{taskSection("", viewTaskTasks(items), m.today())}}, nil
}

func (m model) today() string {
	return m.now().In(m.location).Format(time.DateOnly)
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
		flexColumn: 1,
		firstGap:   1,
		rows:       rows,
	}}}, nil
}

func (m model) loadBoards() (loadedView, error) {
	items, err := m.dependencies.Boards.List(m.ctx)
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{{
		flexColumn: 2,
		firstGap:   1,
		rows:       boardRows(items),
	}}}, nil
}

func (m model) loadAreas() (loadedView, error) {
	items, err := m.dependencies.Areas.List(m.ctx, area.ListOptions{Slice: area.ListSliceActive})
	if err != nil {
		return loadedView{}, err
	}
	return loadedView{sections: []section{{
		flexColumn: 1,
		firstGap:   1,
		rows:       areaRows(items),
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
			entitySection("projects", projectRows(projects)),
			taskSection("tasks", tasks, m.today()),
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
		sections: []section{taskSection("", tasks, m.today())},
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
			flexColumn: 1,
			firstGap:   1,
			hideEmpty:  true,
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
		sections: []section{entitySection("", projectRows(loose))},
	}, nil
}

func (m model) loadTaskDetail(id int64) (loadedView, error) {
	shown, err := m.dependencies.Tasks.Show(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	detail := taskDetail(shown)
	return loadedView{detail: &detail}, nil
}

func (m model) loadProjectDetail(id int64) (loadedView, error) {
	shown, err := m.dependencies.Projects.Show(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	detail := projectDetail(shown)
	return loadedView{detail: &detail}, nil
}

func (m model) loadAreaDetail(id int64) (loadedView, error) {
	shown, err := m.dependencies.Areas.Show(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	detail := areaDetail(shown)
	return loadedView{detail: &detail}, nil
}

func (m model) loadBoardDetail(id int64) (loadedView, error) {
	shown, err := m.dependencies.Boards.ShowByID(m.ctx, id)
	if err != nil {
		return loadedView{}, err
	}
	detail := boardDetail(shown)
	return loadedView{detail: &detail}, nil
}

func (m model) applyLoad(message loadResultMsg) model {
	current := m.top()
	if current.key != message.key || current.generation != message.generation {
		return m
	}
	current.loading = false
	current.err = message.err
	current.loadedView = message.loadedView
	if current.header != nil && len(current.header.cells) > 0 {
		current.crumb = current.header.cells[0]
	}
	state := m.cursors[current.key]
	current.cursor = restoreCursor(current.selectableRows(), state)
	m.ensureCursorVisible(current)
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

func (m model) canFilter(current *frame) bool {
	return !current.loading && current.err == nil && current.detail == nil
}

func (m *model) startFilter(current *frame) {
	if !current.filter.enabled {
		selected, _ := current.selectedRow()
		current.filter = filterState{
			enabled:      true,
			editing:      true,
			input:        m.newFilterInput(),
			origin:       cursorState{identity: selected.identity, index: current.cursor},
			originOffset: current.offset,
		}
	} else {
		current.filter.editing = true
		current.filter.input.Focus()
	}
	m.resizeFilterInput(current)
}

func (m *model) clearFilter(current *frame) {
	origin := current.filter.origin
	offset := current.filter.originOffset
	current.filter = filterState{}
	current.cursor = restoreCursor(current.selectableRows(), origin)
	current.offset = offset
	m.ensureCursorVisible(current)
	m.rememberCursor(current)
}

func (m *model) filterChanged(current *frame, selectedIdentity string, previousIndex int) {
	rows := current.selectableRows()
	current.cursor = rowIndex(rows, selectedIdentity)
	if current.cursor < 0 {
		current.cursor = clampCursor(previousIndex, len(rows))
	}
	m.ensureCursorVisible(current)
}

func (f frame) filteredView() loadedView {
	if !f.filter.enabled || f.filter.input.Value() == "" {
		return f.loadedView
	}
	filtered := loadedView{header: f.header, detail: f.detail, sections: make([]section, len(f.sections))}
	if f.header != nil {
		header := *f.header
		if matches := matchRows(f.filter.input.Value(), []string{filterText(header)}); len(matches) == 1 {
			header.matchPositions = cellMatchPositions(header, matches[0].positions)
		}
		filtered.header = &header
	}
	candidates := make([]string, 0)
	locations := make([][2]int, 0)
	for sectionIndex, current := range f.sections {
		filtered.sections[sectionIndex] = current
		filtered.sections[sectionIndex].rows = nil
		for rowIndex, currentRow := range current.rows {
			candidates = append(candidates, filterText(currentRow))
			locations = append(locations, [2]int{sectionIndex, rowIndex})
		}
	}
	for _, match := range matchRows(f.filter.input.Value(), candidates) {
		location := locations[match.index]
		matchedRow := f.sections[location[0]].rows[location[1]]
		matchedRow.matchPositions = cellMatchPositions(matchedRow, match.positions)
		filtered.sections[location[0]].rows = append(filtered.sections[location[0]].rows, matchedRow)
	}
	return filtered
}

func (f frame) selectableRows() []row {
	if f.detail != nil {
		return nil
	}
	view := f.filteredView()
	count := 0
	includeHeader := view.header != nil && (!f.filter.enabled || f.filter.input.Value() == "" || headerMatched(*view.header))
	if includeHeader {
		count++
	}
	for _, current := range view.sections {
		count += len(current.rows)
	}
	rows := make([]row, 0, count)
	if includeHeader {
		rows = append(rows, *view.header)
	}
	for _, current := range view.sections {
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
	if index := rowIndex(rows, state.identity); index >= 0 {
		return index
	}
	return clamp(state.index, 0, len(rows)-1)
}

func rowIndex(rows []row, identity string) int {
	if identity != "" {
		for index := range rows {
			if rows[index].identity == identity {
				return index
			}
		}
	}
	return -1
}

func clampCursor(index, rowCount int) int {
	if rowCount == 0 {
		return 0
	}
	return clamp(index, 0, rowCount-1)
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}

func rootView() loadedView {
	return loadedView{sections: []section{{rows: rootRows()}}}
}

func rootRows() []row {
	return []row{
		descendingRow("root:inbox", "Inbox", []string{"Inbox"}, viewKey{kind: viewInbox}),
		descendingRow("root:available", "Available", []string{"Available"}, viewKey{kind: viewAvailable}),
		descendingRow("root:logbook", "Logbook", []string{"Logbook"}, viewKey{kind: viewLogbook}),
		descendingRow("root:boards", "Boards", []string{"Boards"}, viewKey{kind: viewBoards}),
		descendingRow("root:areas", "Areas", []string{"Areas"}, viewKey{kind: viewAreas}),
	}
}

func viewTaskTasks(items []task.ViewTask) []task.Task {
	tasks := make([]task.Task, 0, len(items))
	for _, item := range items {
		tasks = append(tasks, item.Task)
	}
	return tasks
}

func taskRows(items []task.Task, today string) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		id := strconv.FormatInt(item.ID, 10)
		current := descendingRow(
			"task:"+id,
			item.Title,
			[]string{"•", taskTitle(item), taskDates(item)},
			viewKey{kind: viewTaskDetail, id: item.ID},
		)
		current.accents = []cellAccent{accentPlain, accentPlain, taskDatesAccent(item, today)}
		rows = append(rows, current)
	}
	return rows
}

func taskDatesAccent(current task.Task, today string) cellAccent {
	if current.DueOn == nil {
		return accentDim
	}
	if current.Status == string(task.ListStatusOpen) && *current.DueOn <= today {
		return accentOverdue
	}
	return accentYellow
}

func logbookRows(entries []logbook.Entry, location *time.Location) ([]row, error) {
	rows := make([]row, 0, len(entries))
	for _, entry := range entries {
		resolvedAt, err := time.Parse(time.RFC3339Nano, entry.ResolvedAt)
		if err != nil {
			return nil, fmt.Errorf("parse logbook resolved_at for %s %d: %w", entry.Kind, entry.ID, err)
		}
		id := strconv.FormatInt(entry.ID, 10)
		var destination viewKey
		switch entry.Kind {
		case "task":
			destination = viewKey{kind: viewTaskDetail, id: entry.ID}
		case "project":
			destination = viewKey{kind: viewProjectDetail, id: entry.ID}
		default:
			return nil, fmt.Errorf("unknown logbook entry kind %q", entry.Kind)
		}
		glyph, glyphAccent := logbookGlyph(entry)
		current := descendingRow(
			entry.Kind+":"+id,
			entry.Title,
			[]string{
				glyph,
				entry.Title,
				entry.Kind,
				resolvedAt.In(location).Format(time.DateOnly),
			},
			destination,
		)
		current.accents = []cellAccent{glyphAccent, accentPlain, accentDim, accentDim}
		rows = append(rows, current)
	}
	return rows, nil
}

func logbookGlyph(entry logbook.Entry) (string, cellAccent) {
	switch entry.Status {
	case "done":
		return "✓", accentGreen
	case "cancelled":
		return "✗", accentRed
	}
	if entry.Kind == "project" {
		return "◆", accentPlain
	}
	return "•", accentPlain
}

func boardRows(items []board.ListedBoard) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		stageTitles := make([]string, len(item.Stages))
		for index := range item.Stages {
			stageTitles[index] = item.Stages[index].Title
		}
		current := descendingRow(
			"board:"+strconv.FormatInt(item.ID, 10),
			item.Title,
			[]string{"▥", item.Title, joinStages(stageTitles)},
			viewKey{kind: viewBoard, id: item.ID},
		)
		current.accents = []cellAccent{accentPlain, accentPlain, accentDim}
		rows = append(rows, current)
	}
	return rows
}

func areaRows(items []area.Area) []row {
	rows := make([]row, 0, len(items)+1)
	for _, item := range items {
		id := strconv.FormatInt(item.ID, 10)
		rows = append(rows, descendingRow(
			"area:"+id,
			item.Title,
			[]string{"●", item.Title},
			viewKey{kind: viewArea, id: item.ID},
		))
	}
	pseudo := descendingRow(
		"area:none",
		"(no area)",
		[]string{"○", "(no area)"},
		viewKey{kind: viewNoArea},
	)
	pseudo.accents = []cellAccent{accentDim, accentDim}
	return append(rows, pseudo)
}

func projectRows(items []project.Project) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		id := strconv.FormatInt(item.ID, 10)
		rows = append(rows, descendingRow(
			"project:"+id,
			item.Title,
			[]string{"◆", item.Title},
			viewKey{kind: viewProject, id: item.ID},
		))
	}
	return rows
}

func boardProjectRows(items []board.ShownProject) []row {
	rows := make([]row, 0, len(items))
	for _, item := range items {
		id := strconv.FormatInt(item.ID, 10)
		current := descendingRow(
			"project:"+id,
			item.Title,
			[]string{
				"◆",
				item.Title,
				fmt.Sprintf("%d/%d", item.Progress.Done, item.Progress.Total),
			},
			viewKey{kind: viewProject, id: item.ID},
		)
		current.accents = []cellAccent{accentPlain, accentPlain, accentDim}
		rows = append(rows, current)
	}
	return rows
}

func descendingRow(identity, crumb string, cells []string, destination viewKey) row {
	return row{identity: identity, crumb: crumb, cells: cells, destination: destination}
}

func containerHeader(style rowStyle, id int64, title string) row {
	kind := viewAreaDetail
	switch style {
	case rowProjectHeader:
		kind = viewProjectDetail
	case rowBoardHeader:
		kind = viewBoardDetail
	}
	header := descendingRow(
		"header:"+strconv.FormatInt(id, 10),
		title,
		[]string{title},
		viewKey{kind: kind, id: id},
	)
	header.style = style
	return header
}

func taskSection(title string, tasks []task.Task, today string) section {
	return section{
		title:      title,
		flexColumn: 1,
		firstGap:   1,
		rows:       taskRows(tasks, today),
	}
}

func entitySection(title string, rows []row) section {
	return section{
		title:      title,
		flexColumn: 1,
		firstGap:   1,
		rows:       rows,
	}
}
