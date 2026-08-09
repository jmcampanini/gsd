package navigator

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/text"
)

const selectionMarkerWidth = 2

func (m model) View() tea.View {
	current := m.top()
	var content string
	switch {
	case current.loading:
		content = m.fit("  " + m.dim("loading…"))
	case current.err != nil:
		content = m.fit(m.red("! ") + text.Human(current.err.Error(), false))
	case current.key.kind == viewRoot:
		content = m.renderRoot(current)
	default:
		content = m.renderFrame(current)
	}
	if content != "" {
		content += "\n"
	}
	return tea.View{Content: content}
}

func (m model) renderRoot(current *frame) string {
	rows := current.selectableRows()
	lines := make([]string, len(rows))
	for index := range rows {
		line := m.marker(index == current.cursor) + text.Human(rows[index].cells[0], false)
		lines[index] = m.fit(line)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderFrame(current *frame) string {
	lines := make([]string, 0, len(current.selectableRows())+len(current.sections)+2)
	selectionOffset := 0
	if current.plainTitle != "" {
		lines = append(lines, m.fit("  "+text.Human(current.plainTitle, false)))
	}
	if current.header != nil {
		line := m.marker(current.cursor == selectionOffset) + renderHeader(*current.header)
		lines = append(lines, m.fit(line))
		selectionOffset++
	}
	for _, currentSection := range current.sections {
		sectionLines := m.renderSection(currentSection, current.cursor, selectionOffset)
		lines = append(lines, sectionLines...)
		selectionOffset += len(currentSection.rows)
	}
	return strings.Join(lines, "\n")
}

func (m model) renderSection(current section, cursor, selectionOffset int) []string {
	lines := make([]string, 0, len(current.rows)+2)
	if current.title != "" {
		lines = append(lines, m.fit("  "+m.dim(text.Human(current.title, false))))
	}
	if len(current.rows) == 0 {
		if current.showEmpty {
			lines = append(lines, m.fit("  "+m.dim("(empty)")))
		}
		return lines
	}

	visibleRows := make([][]string, len(current.rows))
	columnCount := len(current.columns)
	for rowIndex := range current.rows {
		visibleRows[rowIndex] = visibleCells(current.rows[rowIndex].cells)
		columnCount = max(columnCount, len(visibleRows[rowIndex]))
	}
	visibleHeaders := []string(nil)
	if current.showColumns {
		visibleHeaders = visibleCells(current.columns)
	}
	widths := tableWidths(visibleHeaders, visibleRows, columnCount)
	widths = m.fitTableWidths(widths, current.flexColumn, current.firstGap)

	if current.showColumns {
		header := renderCells(visibleHeaders, widths, current.rightAlign, current.firstGap)
		lines = append(lines, m.fit("  "+m.dim(header)))
	}
	for index := range current.rows {
		cells := renderCells(visibleRows[index], widths, current.rightAlign, current.firstGap)
		line := m.marker(cursor == selectionOffset+index) + cells
		lines = append(lines, m.fit(line))
	}
	return lines
}

func renderHeader(current row) string {
	cells := visibleCells(current.cells)
	switch current.style {
	case rowAreaHeader:
		return "● " + cells[0] + "  " + cells[1]
	case rowProjectHeader:
		return "◆ " + cells[0] + "  " + cells[1]
	case rowBoardHeader:
		return cells[1]
	default:
		return strings.Join(cells, "  ")
	}
}

func visibleCells(cells []string) []string {
	visible := make([]string, len(cells))
	for index := range cells {
		visible[index] = text.Human(cells[index], false)
	}
	return visible
}

func tableWidths(headers []string, rows [][]string, columnCount int) []int {
	widths := make([]int, columnCount)
	for index := range headers {
		widths[index] = lipgloss.Width(headers[index])
	}
	for rowIndex := range rows {
		for columnIndex := range rows[rowIndex] {
			widths[columnIndex] = max(widths[columnIndex], lipgloss.Width(rows[rowIndex][columnIndex]))
		}
	}
	return widths
}

func (m model) fitTableWidths(widths []int, flexColumn, firstGap int) []int {
	if m.width <= 0 || flexColumn < 0 || flexColumn >= len(widths) {
		return widths
	}
	available := max(m.width-selectionMarkerWidth, 0)
	overflow := renderedCellsWidth(widths, firstGap) - available
	if overflow <= 0 {
		return widths
	}
	fitted := append([]int(nil), widths...)
	reducible := max(fitted[flexColumn]-1, 0)
	fitted[flexColumn] -= min(reducible, overflow)
	return fitted
}

func renderedCellsWidth(widths []int, firstGap int) int {
	width := 0
	for index, current := range widths {
		width += current
		if index < len(widths)-1 {
			width += columnGap(index, firstGap)
		}
	}
	return width
}

func renderCells(cells []string, widths []int, rightAlign, firstGap int) string {
	var rendered strings.Builder
	for index, cell := range cells {
		cell = text.Ellipsize(cell, widths[index])
		width := lipgloss.Width(cell)
		if index == rightAlign {
			rendered.WriteString(strings.Repeat(" ", widths[index]-width))
		}
		rendered.WriteString(cell)
		if index != rightAlign {
			rendered.WriteString(strings.Repeat(" ", widths[index]-width))
		}
		if index < len(cells)-1 {
			rendered.WriteString(strings.Repeat(" ", columnGap(index, firstGap)))
		}
	}
	return strings.TrimRight(rendered.String(), " ")
}

func columnGap(index, firstGap int) int {
	if index == 0 && firstGap > 0 {
		return firstGap
	}
	return 2
}

func (m model) fit(value string) string {
	if m.width <= 0 {
		return value
	}
	return text.Ellipsize(value, m.width)
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
