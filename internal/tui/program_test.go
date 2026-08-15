package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type staticModel struct{}

type stateModel struct {
	state string
}

type fakeProgram struct {
	model tea.Model
	err   error
}

func (p fakeProgram) Run() (tea.Model, error) {
	return p.model, p.err
}

func (staticModel) Init() tea.Cmd {
	return nil
}

func (m staticModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (staticModel) View() tea.View {
	return tea.NewView("capture")
}

func TestRunProgramReturnsFinalInnerModel(t *testing.T) {
	final := stateModel{state: "final"}
	got, err := runProgram(fakeProgram{model: programModel{model: final}})
	if err != nil {
		t.Fatalf("runProgram() error = %v", err)
	}
	if got != final {
		t.Fatalf("runProgram() model = %#v, want final inner model %#v", got, final)
	}
	if _, leaked := got.(programModel); leaked {
		t.Fatal("runProgram() leaked programModel wrapper")
	}
}

func TestRunProgramRejectsUnexpectedWrapper(t *testing.T) {
	_, err := runProgram(fakeProgram{model: staticModel{}})
	if err == nil || !strings.Contains(err.Error(), "unexpected program model") {
		t.Fatalf("runProgram() error = %v, want unexpected program model", err)
	}
}

func TestRunProgramPropagatesProgramError(t *testing.T) {
	failure := errors.New("program failed")
	tests := []struct {
		name  string
		model tea.Model
	}{
		{name: "absent final model"},
		{name: "unexpected final model", model: staticModel{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := runProgram(fakeProgram{model: test.model, err: failure})
			if !errors.Is(err, failure) {
				t.Fatalf("runProgram() error = %v, want %v", err, failure)
			}
		})
	}
}

func TestRunProgramReturnsFinalInnerModelWithProgramError(t *testing.T) {
	failure := errors.New("program failed")
	final := stateModel{state: "final"}
	got, err := runProgram(fakeProgram{
		model: programModel{model: final},
		err:   failure,
	})
	if got != final {
		t.Fatalf("runProgram() model = %#v, want final inner model %#v", got, final)
	}
	if !errors.Is(err, failure) {
		t.Fatalf("runProgram() error = %v, want %v", err, failure)
	}
}

func (stateModel) Init() tea.Cmd {
	return nil
}

func (m stateModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (stateModel) View() tea.View {
	return tea.NewView("")
}

func TestProgramModelAppliesScreenMode(t *testing.T) {
	tests := []struct {
		name      string
		screen    ScreenMode
		altScreen bool
	}{
		{name: "inline", screen: ScreenInline},
		{name: "alternate", screen: ScreenAlt, altScreen: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := programModel{model: staticModel{}, screen: test.screen}
			if got := model.View().AltScreen; got != test.altScreen {
				t.Errorf("AltScreen = %t, want %t", got, test.altScreen)
			}
		})
	}
}
