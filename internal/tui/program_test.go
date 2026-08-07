package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

type staticModel struct{}

func (staticModel) Init() tea.Cmd {
	return nil
}

func (m staticModel) Update(tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (staticModel) View() tea.View {
	return tea.NewView("capture")
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
