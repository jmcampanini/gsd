package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/text"
)

const (
	captureFooter       = "enter add · esc cancel"
	captureAddingStatus = "adding · esc cancel"
	captureCancelStatus = "canceling"
	cursorCellWidth     = 1
)

type capturePhase uint8

const (
	captureTyping capturePhase = iota
	captureSubmitting
	captureCanceling
	captureErrored
	captureQuitting
)

type CaptureModel struct {
	ctx          context.Context
	application  task.Application
	input        textinput.Model
	theme        Theme
	footerStyle  lipgloss.Style
	errorStyle   lipgloss.Style
	colorEnabled bool
	width        int
	phase        capturePhase
	err          error
	submission   *captureSubmission
}

func newCaptureModel(
	ctx context.Context,
	application task.Application,
	colorEnabled bool,
	submission *captureSubmission,
) CaptureModel {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Focus()

	model := CaptureModel{
		ctx:          ctx,
		application:  application,
		input:        input,
		colorEnabled: colorEnabled,
		submission:   submission,
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
		m.submission.cancel()
		if msg.err == nil || (m.phase == captureCanceling && errors.Is(msg.err, context.Canceled)) {
			m.phase = captureQuitting
			return m, tea.Quit
		}
		m.phase = captureErrored
		m.err = msg.err
		return m, nil
	case tea.KeyPressMsg:
		if m.phase == captureErrored {
			return m, tea.Quit
		}
		if m.phase == captureQuitting {
			return m, nil
		}

		key := msg.String()
		if key == "ctrl+c" || key == "esc" {
			if m.phase == captureSubmitting {
				m.phase = captureCanceling
				m.submission.cancel()
			}
			if m.phase == captureCanceling {
				return m, nil
			}
			return m, tea.Quit
		}
		if m.phase != captureTyping {
			return m, nil
		}
		if key == "enter" {
			title := m.input.Value()
			if strings.TrimSpace(title) == "" {
				return m, nil
			}
			command := m.submission.register(m.ctx, m.application, title)
			m.phase = captureSubmitting
			m.input.Blur()
			return m, command
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m CaptureModel) View() tea.View {
	view := tea.NewView(m.inputView() + "\n" + m.footerView())
	view.Cursor = m.input.Cursor()
	return view
}

func (m CaptureModel) footerView() string {
	switch m.phase {
	case captureErrored:
		message := "Error: " + text.Human(m.err.Error(), false)
		if m.width > 0 {
			_, right, _, left := m.errorStyle.GetPadding()
			message = text.Ellipsize(message, max(m.width-left-right, 0))
		}
		return m.errorStyle.Render(message)
	case captureCanceling:
		return m.footerStyle.Render(captureCancelStatus)
	case captureSubmitting:
		return m.footerStyle.Render(captureAddingStatus)
	default:
		return m.footerStyle.Render(captureFooter)
	}
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

type captureSubmission struct {
	mutex         sync.Mutex
	cancelContext context.CancelFunc
	done          chan struct{}
	started       bool
	finished      bool
	err           error
}

// register exposes the submission to shutdown before Bubble Tea can schedule its command.
func (s *captureSubmission) register(
	parent context.Context,
	application task.Application,
	title string,
) tea.Cmd {
	ctx, cancel := context.WithCancel(parent)
	s.mutex.Lock()
	s.cancelContext = cancel
	s.done = make(chan struct{})
	s.mutex.Unlock()

	return func() tea.Msg {
		s.mutex.Lock()
		if s.finished {
			err := s.err
			s.mutex.Unlock()
			return captureResultMsg{err: err}
		}
		s.started = true
		s.mutex.Unlock()

		var err error
		defer func() { s.finish(err) }()
		_, err = application.Add(ctx, task.AddRequest{Title: title})
		return captureResultMsg{err: err}
	}
}

func (s *captureSubmission) cancel() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancelContext != nil {
		s.cancelContext()
	}
}

func (s *captureSubmission) finish(err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.err = err
	s.finished = true
	close(s.done)
}

func (s *captureSubmission) cancelAndWait() error {
	s.mutex.Lock()
	if s.done == nil {
		s.mutex.Unlock()
		return nil
	}
	s.cancelContext()
	if !s.started && !s.finished {
		s.err = context.Canceled
		s.finished = true
		close(s.done)
	}
	done := s.done
	s.mutex.Unlock()

	<-done
	return s.result()
}

func (s *captureSubmission) result() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.err
}

type captureProgramRunner func(CaptureModel) (CaptureModel, error)

func RunCapture(
	ctx context.Context,
	application task.Application,
	options ProgramOptions,
) error {
	return runCapture(
		ctx,
		application,
		options.Color != ColorDisabled,
		func(model CaptureModel) (CaptureModel, error) {
			finalModel, err := RunProgram(ctx, model, options)
			if err != nil {
				return CaptureModel{}, err
			}

			capture, ok := finalModel.(CaptureModel)
			if !ok {
				return CaptureModel{}, fmt.Errorf("unexpected capture model %T", finalModel)
			}
			return capture, nil
		},
	)
}

func runCapture(
	ctx context.Context,
	application task.Application,
	colorEnabled bool,
	runProgram captureProgramRunner,
) error {
	submission := &captureSubmission{}
	// A panic in runProgram must not unwind into the command's database close
	// while an Add is still in flight.
	defer func() { _ = submission.cancelAndWait() }()

	capture, programErr := runProgram(
		newCaptureModel(ctx, application, colorEnabled, submission),
	)
	submissionErr := submission.cancelAndWait()
	if programErr != nil {
		return programErr
	}
	switch capture.phase {
	case captureSubmitting:
		return submissionErr
	case captureCanceling:
		if errors.Is(submissionErr, context.Canceled) {
			return nil
		}
		return submissionErr
	default:
		return capture.Err()
	}
}
