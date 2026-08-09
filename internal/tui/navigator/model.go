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
	Show(context.Context, string) (board.Show, error)
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
)

type viewKey struct {
	kind viewKind
}

type row struct {
	identity string
	cells    []string
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
	columns    []string
	rightAlign int
	rows       []row
	cursor     int
}

type loadResultMsg struct {
	key        viewKey
	generation uint64
	columns    []string
	rightAlign int
	rows       []row
	err        error
}

type model struct {
	ctx          context.Context
	dependencies Dependencies
	location     *time.Location
	theme        tui.Theme
	colorEnabled bool
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
	root := frame{key: rootKey, rows: rootRows(), rightAlign: -1}
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
	case "enter":
		if m.top().key.kind == viewRoot {
			return m.pushRootSelection()
		}
	}
	return m, nil
}

func (m *model) move(delta int) {
	current := m.top()
	if current.loading || current.err != nil || len(current.rows) == 0 {
		return
	}
	current.cursor = clamp(current.cursor+delta, 0, len(current.rows)-1)
	m.rememberCursor(current)
}

func (m model) pushRootSelection() (tea.Model, tea.Cmd) {
	current := m.top()
	if len(current.rows) == 0 {
		return m, nil
	}
	kind := viewKind(current.cursor + int(viewInbox))
	m.rememberCursor(current)
	m.stack = append(m.stack, frame{key: viewKey{kind: kind}, rightAlign: -1})
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
		current.rows = rootRows()
		current.cursor = restoreCursor(current.rows, state)
		m.rememberCursor(current)
		return m, nil
	}

	m.generation++
	current.generation = m.generation
	current.loading = true
	current.err = nil
	current.columns = nil
	current.rows = nil
	current.cursor = state.index
	generation := current.generation
	key := current.key
	return m, m.loadCommand(key, generation)
}

func (m model) loadCommand(key viewKey, generation uint64) tea.Cmd {
	return func() tea.Msg {
		result := loadResultMsg{key: key, generation: generation, rightAlign: -1}
		switch key.kind {
		case viewInbox:
			items, err := m.dependencies.Tasks.Inbox(m.ctx)
			result.columns = []string{"id", "title", "dates"}
			result.rightAlign = 0
			result.rows = taskRows(items)
			result.err = err
		case viewAvailable:
			items, err := m.dependencies.Tasks.Available(m.ctx)
			result.columns = []string{"id", "title", "dates"}
			result.rightAlign = 0
			result.rows = taskRows(items)
			result.err = err
		case viewLogbook:
			entries, err := m.dependencies.Logbook.List(m.ctx)
			result.columns = []string{"kind", "id", "title", "status", "date"}
			result.rightAlign = 1
			if err == nil {
				result.rows, result.err = logbookRows(entries, m.location)
			} else {
				result.err = err
			}
		case viewBoards:
			boards, err := m.dependencies.Boards.List(m.ctx)
			result.columns = []string{"board", "stages"}
			if err == nil {
				result.rows = boardRows(boards)
			} else {
				result.err = err
			}
		case viewAreas:
			areas, err := m.dependencies.Areas.List(m.ctx, area.ListOptions{Slice: area.ListSliceActive})
			result.columns = []string{"id", "title", "state"}
			result.rightAlign = 0
			if err == nil {
				result.rows = areaRows(areas)
			} else {
				result.err = err
			}
		case viewRoot:
			result.err = fmt.Errorf("root view has no loader")
		}
		if result.err != nil {
			result.rows = nil
		}
		return result
	}
}

func (m model) applyLoad(message loadResultMsg) model {
	current := m.top()
	if current.key != message.key || current.generation != message.generation {
		return m
	}
	current.loading = false
	current.err = message.err
	current.columns = message.columns
	current.rightAlign = message.rightAlign
	current.rows = message.rows
	state := m.cursors[current.key]
	current.cursor = restoreCursor(current.rows, state)
	m.rememberCursor(current)
	return m
}

func (m *model) rememberCursor(current *frame) {
	state := m.cursors[current.key]
	state.index = current.cursor
	if len(current.rows) > 0 && current.cursor >= 0 && current.cursor < len(current.rows) {
		state.identity = current.rows[current.cursor].identity
	}
	m.cursors[current.key] = state
}

func (m *model) top() *frame {
	return &m.stack[len(m.stack)-1]
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
		{identity: "root:inbox", cells: []string{"Inbox"}},
		{identity: "root:available", cells: []string{"Available"}},
		{identity: "root:logbook", cells: []string{"Logbook"}},
		{identity: "root:boards", cells: []string{"Boards"}},
		{identity: "root:areas", cells: []string{"Areas"}},
	}
}

func taskRows(items []task.ViewTask) []row {
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
		rows = append(rows, row{
			identity: "board:" + strconv.FormatInt(item.ID, 10),
			cells:    []string{item.Title, joinStages(stageTitles)},
		})
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
		rows = append(rows, row{
			identity: "area:" + strconv.FormatInt(item.ID, 10),
			cells:    []string{strconv.FormatInt(item.ID, 10), item.Title, state},
		})
	}
	return append(rows, row{identity: "area:none", cells: []string{"", "(no area)", ""}})
}
