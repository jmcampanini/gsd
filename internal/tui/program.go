package tui

import (
	"context"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// ScreenMode chooses whether a program draws inline or in the alternate
// screen.
type ScreenMode uint8

// The screen modes. ScreenInline draws at the cursor; ScreenAlt takes over
// the terminal and restores it on exit.
const (
	ScreenInline ScreenMode = iota
	ScreenAlt
)

// ProgramOptions configures a Bubble Tea program: its streams, the
// environment it reads, the screen mode, and the resolved color profile.
type ProgramOptions struct {
	Input       io.Reader
	Output      io.Writer
	Environment []string
	Screen      ScreenMode
	Profile     colorprofile.Profile
	Terminal    bool
}

// ColorEnabled reports whether the program should render color.
func (o ProgramOptions) ColorEnabled() bool {
	return CanRenderColor(o.Profile, o.Terminal)
}

// CanRenderColor reports whether output is a terminal whose profile supports
// at least ANSI color.
func CanRenderColor(profile colorprofile.Profile, terminal bool) bool {
	return terminal && profile >= colorprofile.ANSI
}

type runnableProgram interface {
	Run() (tea.Model, error)
}

// RunProgram runs model to completion and returns its final state, unwrapped
// from the screen-mode wrapper NewProgram adds.
func RunProgram(ctx context.Context, model tea.Model, options ProgramOptions) (tea.Model, error) {
	return runProgram(NewProgram(ctx, model, options))
}

func runProgram(program runnableProgram) (tea.Model, error) {
	finalModel, programErr := program.Run()
	configured, ok := finalModel.(programModel)
	if !ok {
		if programErr != nil {
			return nil, programErr
		}
		return nil, fmt.Errorf("unexpected program model %T", finalModel)
	}
	return configured.model, programErr
}

// NewProgram builds the program for model with options, wrapping model so
// every view carries the chosen screen mode.
func NewProgram(ctx context.Context, model tea.Model, options ProgramOptions) *tea.Program {
	programOptions := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(options.Input),
		tea.WithOutput(options.Output),
		tea.WithColorProfile(options.Profile),
	}
	if options.Environment != nil {
		programOptions = append(programOptions, tea.WithEnvironment(options.Environment))
	}
	return tea.NewProgram(programModel{model: model, screen: options.Screen}, programOptions...)
}

type programModel struct {
	model  tea.Model
	screen ScreenMode
}

func (m programModel) Init() tea.Cmd {
	return m.model.Init()
}

func (m programModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := m.model.Update(msg)
	m.model = updated
	return m, cmd
}

func (m programModel) View() tea.View {
	view := m.model.View()
	view.AltScreen = m.screen == ScreenAlt
	return view
}
