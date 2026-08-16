package navigator

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jmcampanini/gsd/internal/text"
)

const (
	boardColumnFloor  = 20
	boardColumnGutter = 3
	boardColumnMargin = 2
)

type boardColumnLayout struct {
	start       int
	widths      []int
	hiddenLeft  int
	hiddenRight int
}

func calculateBoardColumnLayout(width, columnCount, selected, offset int) boardColumnLayout {
	if columnCount == 0 {
		return boardColumnLayout{}
	}
	available := max(width-2*boardColumnMargin, 0)
	visibleCount := columnCount
	minimum := columnCount*boardColumnFloor + (columnCount-1)*boardColumnGutter
	if available < minimum {
		visibleCount = max((available+boardColumnGutter)/(boardColumnFloor+boardColumnGutter), 1)
		visibleCount = min(visibleCount, columnCount)
	}
	start := clamp(offset, 0, columnCount-visibleCount)
	selected = clamp(selected, 0, columnCount-1)
	if selected < start {
		start = selected
	} else if selected >= start+visibleCount {
		start = selected - visibleCount + 1
	}
	start = clamp(start, 0, columnCount-visibleCount)

	columnSpace := max(available-(visibleCount-1)*boardColumnGutter, 0)
	baseWidth := columnSpace / visibleCount
	remainder := columnSpace % visibleCount
	widths := make([]int, visibleCount)
	for index := range widths {
		widths[index] = baseWidth
		if index < remainder {
			widths[index]++
		}
	}
	return boardColumnLayout{
		start:       start,
		widths:      widths,
		hiddenLeft:  start,
		hiddenRight: columnCount - start - visibleCount,
	}
}

func (m *model) ensureBoardColumnVisible(current *frame) {
	if current.boardColumns == nil || len(current.boardColumns.columns) == 0 {
		current.columnState = boardColumnState{card: -1}
		return
	}
	current.columnState.column = clamp(
		current.columnState.column,
		0,
		len(current.boardColumns.columns)-1,
	)
	layout := calculateBoardColumnLayout(
		m.boardColumnRenderWidth(len(current.boardColumns.columns)),
		len(current.boardColumns.columns),
		current.columnState.column,
		current.columnState.offset,
	)
	current.columnState.offset = layout.start
}

func (m model) renderBoardColumns(current *frame) []string {
	if current.boardColumns == nil || len(current.boardColumns.columns) == 0 {
		return nil
	}
	width := m.boardColumnRenderWidth(len(current.boardColumns.columns))
	height := m.boardColumnRenderHeight(current.boardColumns.columns)
	if height <= 0 {
		return nil
	}
	layout := calculateBoardColumnLayout(
		width,
		len(current.boardColumns.columns),
		current.columnState.column,
		current.columnState.offset,
	)
	columnLines := make([][]string, len(layout.widths))
	for visibleIndex, columnWidth := range layout.widths {
		columnIndex := layout.start + visibleIndex
		columnLines[visibleIndex] = m.renderBoardColumn(
			current.boardColumns.columns[columnIndex],
			columnWidth,
			height,
			columnIndex == current.columnState.column,
			current.columnState.card,
		)
	}

	lines := make([]string, height)
	for lineIndex := range lines {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", boardColumnMargin))
		for columnIndex := range columnLines {
			if columnIndex > 0 {
				line.WriteString(" ")
				line.WriteString(m.faint("│"))
				line.WriteString(" ")
			}
			line.WriteString(columnLines[columnIndex][lineIndex])
		}
		line.WriteString(strings.Repeat(" ", boardColumnMargin))
		lines[lineIndex] = fitBoardColumnLine(line.String(), width)
	}
	m.renderBoardColumnEdges(lines, layout, width)
	return lines
}

func (m model) renderBoardColumn(
	column boardColumn,
	width, height int,
	selectedColumn bool,
	selectedCard int,
) []string {
	lines := make([]string, height)
	for index := range lines {
		lines[index] = strings.Repeat(" ", width)
	}
	if height == 0 {
		return lines
	}
	lines[0] = m.renderBoardColumnHeading(column.title, width, selectedColumn)
	if height == 1 {
		return lines
	}
	lines[1] = m.faint(strings.Repeat("─", width))

	cardCapacity := max((height-2+1)/3, 0)
	if cardCapacity == 0 {
		return lines
	}
	cardOffset := 0
	if selectedColumn && selectedCard >= cardCapacity {
		cardOffset = selectedCard - cardCapacity + 1
	}
	for visibleIndex := 0; visibleIndex < cardCapacity; visibleIndex++ {
		cardIndex := cardOffset + visibleIndex
		if cardIndex >= len(column.cards) {
			break
		}
		lineIndex := 2 + visibleIndex*3
		if lineIndex+1 >= height {
			break
		}
		card := column.cards[cardIndex]
		selected := selectedColumn && cardIndex == selectedCard
		lines[lineIndex] = m.renderBoardCardLine(card.cells[0], width, accentPlain, selected)
		lines[lineIndex+1] = m.renderBoardCardLine(card.cells[1], width, accentDim, selected)
	}
	return lines
}

func (m model) renderBoardColumnHeading(title string, width int, selected bool) string {
	prefixWidth := min(2, width)
	bodyWidth := max(width-prefixWidth, 0)
	visible := text.Ellipsize(text.Human(title, false), bodyWidth)
	padding := strings.Repeat(" ", max(bodyWidth-lipgloss.Width(visible), 0))
	if m.colorEnabled {
		style := lipgloss.NewStyle().Foreground(m.theme.Dim)
		if selected {
			style = lipgloss.NewStyle().Foreground(m.theme.Text).Bold(true)
		}
		visible = style.Render(visible)
	}
	return strings.Repeat(" ", prefixWidth) + visible + padding
}

func (m model) renderBoardCardLine(value string, width int, accent cellAccent, selected bool) string {
	if width <= 0 {
		return ""
	}
	prefixWidth := min(selectionMarkerWidth, width)
	bodyWidth := width - prefixWidth
	visible := text.Ellipsize(text.Human(value, false), bodyWidth)
	padding := strings.Repeat(" ", max(bodyWidth-lipgloss.Width(visible), 0))
	if !m.colorEnabled {
		prefix := strings.Repeat(" ", prefixWidth)
		if selected {
			prefix = ansi.Cut("▌ ", 0, prefixWidth)
		}
		return prefix + visible + padding
	}
	if !selected {
		return strings.Repeat(" ", prefixWidth) + m.styleCell(visible, accent, false) + padding
	}
	edge := lipgloss.NewStyle().
		Foreground(m.theme.Accent).
		Background(m.theme.InputBg).
		Render(ansi.Cut("▌ ", 0, prefixWidth))
	body := lipgloss.NewStyle().Background(m.theme.InputBg)
	if accent == accentDim {
		body = body.Foreground(m.theme.Dim)
	} else {
		body = body.Foreground(m.theme.Text)
	}
	return edge + body.Render(visible+padding)
}

func (m model) renderBoardColumnEdges(lines []string, layout boardColumnLayout, width int) {
	if width <= 0 || len(lines) == 0 {
		return
	}
	if layout.hiddenLeft > 0 {
		bar := m.dim("░")
		for index := range lines {
			lines[index] = overlayBoardColumnLeft(lines[index], bar, width)
		}
		count := m.accentBold("‹" + strconv.Itoa(layout.hiddenLeft))
		lines[0] = overlayBoardColumnLeft(lines[0], count, width)
	}
	if layout.hiddenRight > 0 {
		bar := m.dim("░")
		for index := range lines {
			lines[index] = overlayBoardColumnRight(lines[index], bar, width)
		}
		count := m.accentBold(strconv.Itoa(layout.hiddenRight) + "›")
		lines[0] = overlayBoardColumnRight(lines[0], count, width)
	}
}

func (m model) boardColumnRenderWidth(columnCount int) int {
	if m.width > 0 {
		return m.width
	}
	return 2*boardColumnMargin + columnCount*boardColumnFloor +
		max(columnCount-1, 0)*boardColumnGutter
}

func (m model) boardColumnRenderHeight(columns []boardColumn) int {
	if height := m.contentHeight(); height > 0 {
		return height
	}
	maximumCards := 0
	for _, column := range columns {
		maximumCards = max(maximumCards, len(column.cards))
	}
	if maximumCards == 0 {
		return 2
	}
	return 2 + maximumCards*2 + maximumCards - 1
}

func fitBoardColumnLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	line = ansi.Cut(line, 0, width)
	if padding := width - lipgloss.Width(line); padding > 0 {
		line += strings.Repeat(" ", padding)
	}
	return line
}

func overlayBoardColumnLeft(line, value string, width int) string {
	valueWidth := lipgloss.Width(value)
	if valueWidth >= width {
		return ansi.Cut(value, 0, width)
	}
	return value + ansi.Cut(line, valueWidth, width)
}

func overlayBoardColumnRight(line, value string, width int) string {
	valueWidth := lipgloss.Width(value)
	if valueWidth >= width {
		return ansi.Cut(value, valueWidth-width, valueWidth)
	}
	return ansi.Cut(line, 0, width-valueWidth) + value
}

func (m model) faint(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Faint(true).Render(value)
}

func (m model) accentBold(value string) string {
	if !m.colorEnabled {
		return value
	}
	return lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true).Render(value)
}
