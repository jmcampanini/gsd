package navigator

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/text"
)

func (m model) View() tea.View {
	current := m.top()
	var content string
	switch {
	case current.loading:
		content = "  " + m.dim("loading…")
	case current.err != nil:
		content = m.red("! ") + text.Human(current.err.Error(), false)
	case current.key.kind == viewRoot:
		content = m.renderRoot(current)
	default:
		content = m.renderRows(current)
	}
	if content != "" {
		content += "\n"
	}
	return tea.View{Content: content}
}

func (m model) renderRoot(current *frame) string {
	lines := make([]string, len(current.rows))
	for index := range current.rows {
		lines[index] = m.marker(index == current.cursor) + text.Human(current.rows[index].cells[0], false)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderRows(current *frame) string {
	if len(current.rows) == 0 {
		return ""
	}
	widths := make([]int, len(current.columns))
	visibleHeaders := visibleCells(current.columns)
	for index := range visibleHeaders {
		widths[index] = lipgloss.Width(visibleHeaders[index])
	}
	visibleRows := make([][]string, len(current.rows))
	for rowIndex := range current.rows {
		visibleRows[rowIndex] = visibleCells(current.rows[rowIndex].cells)
		for columnIndex := range visibleRows[rowIndex] {
			widths[columnIndex] = max(widths[columnIndex], lipgloss.Width(visibleRows[rowIndex][columnIndex]))
		}
	}

	lines := make([]string, 0, len(current.rows)+1)
	lines = append(lines, "  "+m.dim(renderCells(visibleHeaders, widths, current.rightAlign)))
	for index := range current.rows {
		lines = append(lines, m.marker(index == current.cursor)+renderCells(visibleRows[index], widths, current.rightAlign))
	}
	return strings.Join(lines, "\n")
}

func visibleCells(cells []string) []string {
	visible := make([]string, len(cells))
	for index := range cells {
		visible[index] = text.Human(cells[index], false)
	}
	return visible
}

func renderCells(cells []string, widths []int, rightAlign int) string {
	var rendered strings.Builder
	for index, cell := range cells {
		width := lipgloss.Width(cell)
		if index == rightAlign {
			rendered.WriteString(strings.Repeat(" ", widths[index]-width))
		}
		rendered.WriteString(cell)
		if index != rightAlign {
			rendered.WriteString(strings.Repeat(" ", widths[index]-width))
		}
		if index < len(cells)-1 {
			rendered.WriteString("  ")
		}
	}
	return strings.TrimRight(rendered.String(), " ")
}

func (m model) marker(selected bool) string {
	if !selected {
		return "  "
	}
	if !m.colorEnabled {
		return "> "
	}
	return lipgloss.NewStyle().Foreground(m.theme.Accent).Render("> ")
}

func (m model) dim(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Foreground(m.theme.Dim).Render(value)
}

func (m model) red(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Foreground(m.theme.Red).Render(value)
}

func taskTitle(current task.Task) string {
	title := current.Title
	if current.Promotes {
		title += " ↑"
	}
	return title
}

func taskDates(current task.Task) string {
	tokens := make([]string, 0, 3)
	if current.DueOn != nil {
		tokens = append(tokens, "due "+*current.DueOn)
	}
	if current.DeferUntil != nil {
		tokens = append(tokens, "defer "+*current.DeferUntil)
	}
	if current.DeferStageTitle != nil {
		tokens = append(tokens, "defer→"+*current.DeferStageTitle)
	}
	return strings.Join(tokens, " ")
}

func joinStages(stages []string) string {
	return strings.Join(stages, " → ")
}
