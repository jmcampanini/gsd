package tui

import (
	"context"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

type ScreenMode uint8

const (
	ScreenInline ScreenMode = iota
	ScreenAlt
)

type ColorMode uint8

const (
	ColorDisabled ColorMode = iota
	ColorDetected
	ColorForced
)

type ProgramOptions struct {
	Input       io.Reader
	Output      io.Writer
	Environment []string
	Screen      ScreenMode
	Color       ColorMode
}

type runnableProgram interface {
	Run() (tea.Model, error)
}

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

func NewProgram(ctx context.Context, model tea.Model, options ProgramOptions) *tea.Program {
	programOptions := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(options.Input),
		tea.WithOutput(options.Output),
	}
	if options.Environment != nil {
		programOptions = append(programOptions, tea.WithEnvironment(options.Environment))
	}
	switch options.Color {
	case ColorDisabled:
		programOptions = append(programOptions, tea.WithColorProfile(colorprofile.NoTTY))
	case ColorForced:
		programOptions = append(programOptions, tea.WithColorProfile(colorprofile.TrueColor))
	case ColorDetected:
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
