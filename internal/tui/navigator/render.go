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
	var contentLines []string
	switch {
	case current.loading:
		contentLines = []string{m.fit("  " + m.dim("loading…"))}
	case current.err != nil:
		contentLines = []string{m.fit(m.red("! ") + text.Human(current.err.Error(), false))}
	default:
		lines, _ := m.renderLines(current)
		contentLines = m.visibleLines(lines, current.offset)
	}
	content := strings.Join(m.frameLines(contentLines), "\n")
	if content != "" {
		content += "\n"
	}
	return tea.View{Content: content}
}

func (m model) frameLines(contentLines []string) []string {
	if m.height == 1 || m.height == 2 {
		return contentLines
	}
	framed := make([]string, 0, len(contentLines)+4)
	framed = append(framed, m.topBand())
	if m.height <= 0 || m.height >= 4 {
		framed = append(framed, "")
	}
	framed = append(framed, contentLines...)
	if m.height >= 3 {
		for len(framed) < m.height-1 {
			framed = append(framed, "")
		}
	} else {
		framed = append(framed, "")
	}
	return append(framed, m.bottomBand())
}

func (m model) contentHeight() int {
	if m.height <= 0 {
		return 0
	}
	if m.height <= 2 {
		return m.height
	}
	return max(m.height-3, 1)
}

func (m model) topBand() string {
	crumbs := make([]string, 0, len(m.stack)-1)
	for _, current := range m.stack[1:] {
		crumb := text.Human(current.crumb, false)
		if len(crumbs) > 0 && crumbs[len(crumbs)-1] == crumb {
			continue
		}
		crumbs = append(crumbs, crumb)
	}
	available := 0
	if m.width > 0 {
		available = m.width - 7
	}
	crumbs = collapseCrumbs(crumbs, available)
	if !m.colorEnabled {
		line := " gsd"
		if len(crumbs) > 0 {
			line += "  " + strings.Join(crumbs, " ▸ ")
		}
		return m.fit(line)
	}
	band := lipgloss.NewStyle().Background(m.theme.InputBg)
	badge := lipgloss.NewStyle().Background(m.theme.Accent).Foreground(m.theme.AccentText)
	var rendered strings.Builder
	rendered.WriteString(band.Render(" "))
	rendered.WriteString(badge.Render(" gsd "))
	width := 6
	if len(crumbs) > 0 {
		rendered.WriteString(band.Render(" "))
		width++
		if len(crumbs) > 1 {
			parents := strings.Join(crumbs[:len(crumbs)-1], " ▸ ") + " ▸ "
			rendered.WriteString(band.Foreground(m.theme.Dim).Render(parents))
			width += lipgloss.Width(parents)
		}
		last := crumbs[len(crumbs)-1]
		rendered.WriteString(band.Foreground(m.theme.Text).Bold(true).Render(last))
		width += lipgloss.Width(last)
	}
	if m.width > width {
		rendered.WriteString(band.Render(strings.Repeat(" ", m.width-width)))
	}
	return rendered.String()
}

func collapseCrumbs(crumbs []string, available int) []string {
	if len(crumbs) == 0 || available <= 0 || pathWidth(crumbs) <= available {
		return crumbs
	}
	working := append([]string(nil), crumbs...)
	working[0] = "…"
	for len(working) > 2 && pathWidth(working) > available {
		working = append(working[:1], working[2:]...)
	}
	if pathWidth(working) > available {
		last := working[len(working)-1]
		prefix := pathWidth(working) - lipgloss.Width(last)
		working[len(working)-1] = text.Ellipsize(last, max(available-prefix, 1))
	}
	return working
}

func pathWidth(segments []string) int {
	width := 3 * (len(segments) - 1)
	for _, segment := range segments {
		width += lipgloss.Width(segment)
	}
	return width
}

func (m model) bottomBand() string {
	hints := m.hints()
	if m.width > 1 {
		hints = text.Ellipsize(hints, m.width-1)
	}
	if !m.colorEnabled {
		return " " + hints
	}
	band := lipgloss.NewStyle().Background(m.theme.InputBg)
	line := band.Render(" ") + band.Foreground(m.theme.Dim).Render(hints)
	width := 1 + lipgloss.Width(hints)
	if m.width > width {
		line += band.Render(strings.Repeat(" ", m.width-width))
	}
	return line
}

func (m model) hints() string {
	if m.top().err != nil {
		return "esc back"
	}
	switch m.top().key.kind {
	case viewRoot:
		return "j/k move · ⏎ open · esc quit"
	case viewTaskDetail, viewProjectDetail, viewAreaDetail, viewBoardDetail:
		return "j/k scroll · esc back"
	default:
		return "j/k move · ⏎ open · esc back"
	}
}

func (m model) renderLines(current *frame) ([]string, int) {
	if current.detail != nil {
		return m.renderDetail(*current.detail), -1
	}
	if current.key.kind == viewRoot {
		return m.renderRoot(current)
	}
	return m.renderFrame(current)
}

func (m model) renderDetail(current detailView) []string {
	lines := []string{m.fit(m.detailHeadline(current))}
	labelWidth := 0
	for _, field := range current.fields {
		labelWidth = max(labelWidth, lipgloss.Width(field.label))
	}
	indent := strings.Repeat(" ", 4+labelWidth+2)
	for _, field := range current.fields {
		valueLines := strings.Split(text.Human(field.value, field.preserveLineFeeds), "\n")
		label := "    " + m.dim(field.label)
		if valueLines[0] != "" {
			label += strings.Repeat(" ", labelWidth-lipgloss.Width(field.label)) + "  " + valueLines[0]
		}
		lines = append(lines, m.fit(label))
		for _, continuation := range valueLines[1:] {
			if continuation == "" {
				lines = append(lines, "")
				continue
			}
			lines = append(lines, m.fit(indent+continuation))
		}
	}
	return lines
}

func (m model) detailHeadline(current detailView) string {
	title := text.Human(current.title, false)
	if current.promotes {
		title += " ↑"
	}
	glyph := ""
	switch current.kind {
	case detailTask:
		glyph = "•"
	case detailProject:
		glyph = "◆"
	case detailArea:
		glyph = "●"
	case detailBoard:
		glyph = "▥"
	}
	switch current.status {
	case "done":
		glyph = m.green("✓")
	case "cancelled", "archived":
		glyph = m.red("✗")
	}
	return glyph + " " + title
}

func (m model) renderRoot(current *frame) ([]string, int) {
	rows := current.selectableRows()
	lines := make([]string, len(rows))
	for index := range rows {
		selected := index == current.cursor
		body := m.styleCell(text.Human(rows[index].cells[0], false), accentPlain, selected && m.colorEnabled)
		lines[index] = m.renderRow(selected, body)
	}
	if current.cursor < 0 || current.cursor >= len(lines) {
		return lines, -1
	}
	return lines, current.cursor
}

func (m model) renderFrame(current *frame) ([]string, int) {
	lines := make([]string, 0, len(current.selectableRows())+len(current.sections)+2)
	selectedLine := -1
	selectionOffset := 0
	if current.header != nil {
		selected := current.cursor == selectionOffset
		if selected {
			selectedLine = len(lines)
		}
		body := m.styleCell(renderHeader(*current.header), accentPlain, selected && m.colorEnabled)
		lines = append(lines, m.renderRow(selected, body), "")
		selectionOffset++
	}
	for index, currentSection := range current.sections {
		if index > 0 {
			lines = append(lines, "")
		}
		sectionLines, sectionSelection := m.renderSection(currentSection, current.cursor, selectionOffset)
		if sectionSelection >= 0 {
			selectedLine = len(lines) + sectionSelection
		}
		lines = append(lines, sectionLines...)
		selectionOffset += len(currentSection.rows)
	}
	return lines, selectedLine
}

func (m model) renderSection(current section, cursor, selectionOffset int) ([]string, int) {
	lines := make([]string, 0, len(current.rows)+2)
	selectedLine := -1
	indent := ""
	if current.title != "" {
		indent = "  "
		lines = append(lines, m.fit("  "+m.dim(text.Human(current.title, false))))
	}
	if len(current.rows) == 0 {
		if current.title != "" && !current.hideEmpty {
			lines = append(lines, m.fit("  "+indent+m.dim("(empty)")))
		}
		return lines, selectedLine
	}

	visibleRows := make([][]string, len(current.rows))
	columnCount := 0
	for rowIndex := range current.rows {
		visibleRows[rowIndex] = visibleCells(current.rows[rowIndex].cells)
		columnCount = max(columnCount, len(visibleRows[rowIndex]))
	}
	widths := tableWidths(visibleRows, columnCount)
	widths = m.fitTableWidths(widths, current.flexColumn, current.firstGap, len(indent))

	for index := range current.rows {
		selected := cursor == selectionOffset+index
		if selected {
			selectedLine = len(lines)
		}
		cells := m.renderCells(
			visibleRows[index],
			current.rows[index].accents,
			widths,
			current.firstGap,
			selected && m.colorEnabled,
		)
		if indent != "" {
			cells = m.styleCell(indent, accentPlain, selected && m.colorEnabled) + cells
		}
		lines = append(lines, m.renderRow(selected, cells))
	}
	return lines, selectedLine
}

func (m *model) ensureCursorVisible(current *frame) {
	budget := m.contentHeight()
	if budget <= 0 || current.loading || current.err != nil {
		current.offset = 0
		return
	}
	lines, selectedLine := m.renderLines(current)
	current.offset = clamp(current.offset, 0, max(len(lines)-budget, 0))
	if selectedLine < 0 {
		return
	}
	if selectedLine < current.offset {
		current.offset = selectedLine
	}
	if selectedLine >= current.offset+budget {
		current.offset = selectedLine - budget + 1
	}
}

func (m model) visibleLines(lines []string, offset int) []string {
	budget := m.contentHeight()
	if budget <= 0 || len(lines) <= budget {
		return lines
	}
	offset = clamp(offset, 0, len(lines)-budget)
	return lines[offset : offset+budget]
}

func renderHeader(current row) string {
	cells := visibleCells(current.cells)
	switch current.style {
	case rowAreaHeader:
		return "● " + cells[0]
	case rowProjectHeader:
		return "◆ " + cells[0]
	case rowBoardHeader:
		return "▥ " + cells[0]
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

func tableWidths(rows [][]string, columnCount int) []int {
	widths := make([]int, columnCount)
	for rowIndex := range rows {
		for columnIndex := range rows[rowIndex] {
			widths[columnIndex] = max(widths[columnIndex], lipgloss.Width(rows[rowIndex][columnIndex]))
		}
	}
	return widths
}

func (m model) fitTableWidths(widths []int, flexColumn, firstGap, indent int) []int {
	if m.width <= 0 || flexColumn < 0 || flexColumn >= len(widths) {
		return widths
	}
	available := max(m.width-selectionMarkerWidth-indent, 0)
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

func (m model) renderCells(
	cells []string,
	accents []cellAccent,
	widths []int,
	firstGap int,
	selected bool,
) string {
	last := len(cells) - 1
	for last > 0 && cells[last] == "" {
		last--
	}
	var rendered strings.Builder
	for index := 0; index <= last; index++ {
		cell := text.Ellipsize(cells[index], widths[index])
		if index < last {
			padding := widths[index] - lipgloss.Width(cell) + columnGap(index, firstGap)
			cell += strings.Repeat(" ", padding)
		}
		rendered.WriteString(m.styleCell(cell, accentAt(accents, index), selected))
	}
	return rendered.String()
}

func accentAt(accents []cellAccent, index int) cellAccent {
	if index < len(accents) {
		return accents[index]
	}
	return accentPlain
}

func (m model) styleCell(cell string, accent cellAccent, selected bool) string {
	if !m.colorEnabled {
		return cell
	}
	style := lipgloss.NewStyle()
	styled := false
	if selected {
		style = style.Background(m.theme.InputBg)
		styled = true
	}
	switch accent {
	case accentPlain:
		if selected {
			style = style.Foreground(m.theme.Text)
		}
	case accentDim:
		style = style.Foreground(m.theme.Dim)
		styled = true
	case accentGreen:
		style = style.Foreground(m.theme.Green)
		styled = true
	case accentRed:
		style = style.Foreground(m.theme.Red)
		styled = true
	case accentYellow:
		style = style.Foreground(m.theme.Yellow)
		styled = true
	case accentOverdue:
		style = style.Foreground(m.theme.Red).Bold(true)
		styled = true
	}
	if !styled {
		return cell
	}
	return style.Render(cell)
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

func (m model) renderRow(selected bool, body string) string {
	if !selected {
		return m.fit("  " + body)
	}
	if !m.colorEnabled {
		return m.fit("▌ " + body)
	}
	pad := ""
	if m.width > selectionMarkerWidth {
		body = text.Ellipsize(body, m.width-selectionMarkerWidth)
		padding := m.width - selectionMarkerWidth - lipgloss.Width(body)
		if padding > 0 {
			pad = lipgloss.NewStyle().Background(m.theme.InputBg).Render(strings.Repeat(" ", padding))
		}
	}
	edge := lipgloss.NewStyle().Foreground(m.theme.Accent).Background(m.theme.InputBg).Render("▌ ")
	return edge + body + pad
}

func (m model) dim(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Foreground(m.theme.Dim).Render(value)
}

func (m model) green(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Foreground(m.theme.Green).Render(value)
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
