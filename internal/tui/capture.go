package tui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/gsd/internal/task"
)

const (
	captureFooter       = "enter add · esc cancel"
	captureAddingStatus = "adding · esc cancel"
	captureCancelStatus = "canceling"
	captureAddedStatus  = "added"
	cursorCellWidth     = 1
)

type CaptureModel struct {
	ctx             context.Context
	application     task.Application
	input           textinput.Model
	theme           Theme
	footerStyle     lipgloss.Style
	errorStyle      lipgloss.Style
	colorEnabled    bool
	width           int
	submitting      bool
	cancelRequested bool
	cancelAdd       context.CancelFunc
	quitting        bool
	err             error
}

func NewCaptureModel(
	ctx context.Context,
	application task.Application,
	colorEnabled bool,
) CaptureModel {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Focus()

	model := CaptureModel{
		ctx:          ctx,
		application:  application,
		input:        input,
		colorEnabled: colorEnabled,
	}
	model.setTheme(ThemeForBackground(true))
	return model
}

func (m CaptureModel) Init() tea.Cmd {
	if m.colorEnabled {
		return tea.RequestBackgroundColor
	}
	return nil
}

func (m CaptureModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if m.colorEnabled {
			m.setTheme(ThemeForBackground(msg.IsDark()))
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.resizeInput()
		return m, nil
	case captureResultMsg:
		if m.cancelAdd != nil {
			m.cancelAdd()
			m.cancelAdd = nil
		}
		m.submitting = false
		if msg.err == nil || (m.cancelRequested && errors.Is(msg.err, context.Canceled)) {
			m.quitting = true
			return m, tea.Quit
		}
		m.err = msg.err
		return m, nil
	case tea.KeyPressMsg:
		if m.err != nil {
			return m, tea.Quit
		}
		if m.quitting {
			return m, nil
		}
		if m.submitting {
			switch msg.String() {
			case "ctrl+c", "esc":
				if !m.cancelRequested {
					m.cancelRequested = true
					m.cancelAdd()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			title := m.input.Value()
			if strings.TrimSpace(title) == "" {
				return m, nil
			}
			addContext, cancel := context.WithCancel(m.ctx)
			m.submitting = true
			m.cancelAdd = cancel
			m.input.Blur()
			return m, captureTask(addContext, m.application, title)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m CaptureModel) View() tea.View {
	view := tea.NewView(m.inputView() + "\n" + m.footerView())
	view.Cursor = m.inputCursor()
	return view
}

func (m CaptureModel) footerView() string {
	if m.err != nil {
		return m.errorStyle.Render("Error: " + captureHumanText(m.err.Error()))
	}
	if m.cancelRequested {
		return m.footerStyle.Render(captureCancelStatus)
	}
	if m.quitting {
		return m.footerStyle.Render(captureAddedStatus)
	}
	if m.submitting {
		return m.footerStyle.Render(captureAddingStatus)
	}
	return m.footerStyle.Render(captureFooter)
}

func (m CaptureModel) inputCursor() *tea.Cursor {
	cursor := m.input.Cursor()
	if cursor == nil {
		return nil
	}

	// Render a throwaway virtual cursor to locate Bubbles' private viewport offset.
	probe := m.input
	styles := probe.Styles()
	marker := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	styles.Cursor = textinput.CursorStyle{Color: marker}
	probe.SetStyles(styles)
	probe.SetVirtualCursor(true)

	input := probe.View()
	canvas := lipgloss.NewCanvas(lipgloss.Width(input), 1).
		Compose(lipgloss.NewLayer(input))
	for x := range canvas.Width() {
		foreground := canvas.CellAt(x, 0).Style.Fg
		if foreground != nil && color.RGBAModel.Convert(foreground) == marker {
			cursor.X = x
			break
		}
	}
	return cursor
}

func (m CaptureModel) Err() error {
	return m.err
}

func (m CaptureModel) inputView() string {
	input := m.input.View()
	if !m.colorEnabled || m.width == 0 {
		return input
	}

	canvas := lipgloss.NewCanvas(m.width, 1).Compose(lipgloss.NewLayer(input))
	for x := range m.width {
		cell := canvas.CellAt(x, 0)
		if cell.Style.Bg == nil {
			cell.Style.Bg = m.theme.InputBg
		}
	}
	return canvas.Render()
}

func (m *CaptureModel) setTheme(theme Theme) {
	m.theme = theme
	styles := textinput.Styles{
		Cursor: textinput.CursorStyle{
			Color: theme.Cursor,
			Shape: tea.CursorBlock,
			Blink: true,
		},
	}
	badgeStyle := lipgloss.NewStyle().Padding(0, 1)
	m.footerStyle = lipgloss.NewStyle().PaddingLeft(1)
	m.errorStyle = lipgloss.NewStyle().PaddingLeft(1)

	if m.colorEnabled {
		inputStyle := lipgloss.NewStyle().Foreground(theme.Text)
		state := textinput.StyleState{
			Text:        inputStyle,
			Placeholder: inputStyle.Foreground(theme.Dim),
			Suggestion:  inputStyle.Foreground(theme.Dim),
			Prompt:      lipgloss.NewStyle(),
		}
		styles.Focused = state
		styles.Blurred = state
		badgeStyle = badgeStyle.
			Foreground(theme.AccentText).
			Background(theme.Accent)
		m.footerStyle = m.footerStyle.
			Foreground(theme.Dim).
			Faint(true)
		m.errorStyle = m.errorStyle.Foreground(theme.Red)
	}

	m.input.SetStyles(styles)
	m.input.Prompt = badgeStyle.Render("gsd") + " "
	m.resizeInput()
}

func (m *CaptureModel) resizeInput() {
	m.input.SetWidth(max(m.width-lipgloss.Width(m.input.Prompt)-cursorCellWidth, 0))
}

type captureResultMsg struct {
	err error
}

func captureHumanText(value string) string {
	var visible strings.Builder
	visible.Grow(len(value))
	for _, character := range value {
		if unicode.IsControl(character) {
			quoted := strconv.QuoteRune(character)
			visible.WriteString(quoted[1 : len(quoted)-1])
			continue
		}
		visible.WriteRune(character)
	}
	return visible.String()
}

func captureTask(
	ctx context.Context,
	application task.Application,
	title string,
) tea.Cmd {
	return func() tea.Msg {
		_, err := application.Add(ctx, task.AddFields{Title: title})
		return captureResultMsg{err: err}
	}
}

func RunCapture(
	ctx context.Context,
	application task.Application,
	options ProgramOptions,
) error {
	model := NewCaptureModel(ctx, application, options.Color != ColorDisabled)
	finalModel, err := NewProgram(ctx, model, options).Run()
	if err != nil {
		return err
	}

	configured, ok := finalModel.(programModel)
	if !ok {
		return fmt.Errorf("unexpected capture program model %T", finalModel)
	}
	capture, ok := configured.model.(CaptureModel)
	if !ok {
		return fmt.Errorf("unexpected capture model %T", configured.model)
	}
	return capture.Err()
}
