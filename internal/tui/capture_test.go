package tui

import (
	"context"
	"image/color"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/jmcampanini/gsd/internal/task"
)

type captureApplication struct {
	task.Application
	calls  int
	ctx    context.Context
	fields task.AddFields
	err    error
}

func (a *captureApplication) Add(
	ctx context.Context,
	fields task.AddFields,
) (task.Task, error) {
	a.calls++
	a.ctx = ctx
	a.fields = fields
	return task.Task{}, a.err
}

func TestCaptureInitRequestsBackgroundOnlyWithColor(t *testing.T) {
	tests := []struct {
		name        string
		color       bool
		wantCommand bool
	}{
		{name: "colored", color: true, wantCommand: true},
		{name: "plain"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewCaptureModel(context.Background(), &captureApplication{}, test.color)
			if got := model.Init(); (got != nil) != test.wantCommand {
				t.Errorf("Init command present = %t, want %t", got != nil, test.wantCommand)
			}
		})
	}
}

func TestCaptureEnterAddsExactTitleOnce(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "capture")
	application := &captureApplication{}
	model := NewCaptureModel(ctx, application, true)
	title := "  Call plumber  "
	updated, _ := model.Update(tea.KeyPressMsg{Text: title, Code: ' '})
	model = updated.(CaptureModel)
	if model.input.Value() != title {
		t.Fatalf("input value = %q, want %q", model.input.Value(), title)
	}

	updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(CaptureModel)
	if addCommand == nil {
		t.Fatal("Enter command = nil, want task add")
	}
	if application.calls != 0 {
		t.Fatalf("Add calls before command runs = %d, want 0", application.calls)
	}

	_, duplicateCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if duplicateCommand != nil {
		t.Fatal("queued Enter command should be ignored while adding")
	}

	result := addCommand()
	if application.calls != 1 {
		t.Fatalf("Add calls = %d, want 1", application.calls)
	}
	if application.ctx != ctx {
		t.Fatal("Add context does not match capture context")
	}
	wantFields := task.AddFields{Title: title}
	if !reflect.DeepEqual(application.fields, wantFields) {
		t.Fatalf("Add fields = %#v, want %#v", application.fields, wantFields)
	}

	completed, quitCommand := model.Update(result)
	model = completed.(CaptureModel)
	assertQuitCommand(t, quitCommand)
	if model.Err() != nil {
		t.Fatalf("capture error = %v, want nil", model.Err())
	}
}

func TestCaptureBlankEnterDoesNothing(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "empty"},
		{name: "whitespace", value: " \t  "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := &captureApplication{}
			model := NewCaptureModel(context.Background(), application, true)
			model.input.SetValue(test.value)
			wantValue := model.input.Value()

			updated, command := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model = updated.(CaptureModel)

			if command != nil {
				t.Fatal("blank Enter command should be nil")
			}
			if application.calls != 0 {
				t.Fatalf("Add calls = %d, want 0", application.calls)
			}
			if model.input.Value() != wantValue {
				t.Fatalf("input value = %q, want preserved %q", model.input.Value(), wantValue)
			}
		})
	}
}

func TestCaptureCancelDoesNotAdd(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "escape", key: tea.KeyPressMsg{Code: tea.KeyEscape}},
		{name: "control-c", key: tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := &captureApplication{}
			model := NewCaptureModel(context.Background(), application, true)
			model.input.SetValue("Call plumber")

			_, command := model.Update(test.key)

			assertQuitCommand(t, command)
			if application.calls != 0 {
				t.Fatalf("Add calls = %d, want 0", application.calls)
			}
		})
	}
}

func TestCaptureAdaptsThemeToTerminalBackground(t *testing.T) {
	model := NewCaptureModel(context.Background(), &captureApplication{}, true)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 2})
	model = updated.(CaptureModel)

	dark := ThemeForBackground(true)
	assertInputBand(t, model, dark.InputBg)

	updated, command := model.Update(tea.BackgroundColorMsg{Color: color.White})
	model = updated.(CaptureModel)
	if command != nil {
		t.Fatal("background update command should be nil")
	}
	light := ThemeForBackground(false)
	assertInputBand(t, model, light.InputBg)
	assertColor(
		t,
		"light input text",
		model.input.Styles().Focused.Text.GetForeground(),
		light.Text,
	)
}

func TestCaptureColorDisabledUsesPlainStylesAndVisibleCursor(t *testing.T) {
	model := NewCaptureModel(context.Background(), &captureApplication{}, false)
	styles := model.input.Styles()
	noColor := lipgloss.NoColor{}

	if !reflect.DeepEqual(styles.Focused.Text.GetForeground(), noColor) {
		t.Errorf("plain input foreground = %v, want no color", styles.Focused.Text.GetForeground())
	}
	if !reflect.DeepEqual(styles.Focused.Text.GetBackground(), noColor) {
		t.Errorf("plain input background = %v, want no color", styles.Focused.Text.GetBackground())
	}
	if !reflect.DeepEqual(model.footerStyle.GetForeground(), noColor) {
		t.Errorf("plain footer foreground = %v, want no color", model.footerStyle.GetForeground())
	}
	if model.footerStyle.GetFaint() {
		t.Error("plain footer should not be faint")
	}

	view := programModel{model: model, screen: ScreenAlt}.View()
	if view.Cursor == nil {
		t.Fatal("plain capture cursor = nil, want visible terminal cursor")
	}
	if view.Cursor.X != lipgloss.Width(model.input.Prompt) || view.Cursor.Y != 0 {
		t.Errorf(
			"plain capture cursor = (%d, %d), want (%d, 0)",
			view.Cursor.X,
			view.Cursor.Y,
			lipgloss.Width(model.input.Prompt),
		)
	}
	if view.Cursor.Color != nil {
		t.Errorf("plain capture cursor color = %v, want terminal default", view.Cursor.Color)
	}
}

func TestCaptureCursorThemeTokenFlowsToView(t *testing.T) {
	model := NewCaptureModel(context.Background(), &captureApplication{}, true)
	if cursor := model.View().Cursor; cursor == nil || cursor.Color != nil {
		t.Fatalf("default cursor = %#v, want visible terminal-default cursor", cursor)
	}

	theme := ThemeForBackground(true)
	theme.Cursor = lipgloss.Color("#babbf1")
	model.setTheme(theme, true)
	cursor := model.View().Cursor
	if cursor == nil {
		t.Fatal("themed cursor = nil, want visible cursor")
	}
	assertColor(t, "themed cursor", cursor.Color, theme.Cursor)
}

func TestCaptureViewHasBadgeInputAndFooter(t *testing.T) {
	model := NewCaptureModel(context.Background(), &captureApplication{}, true)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 2})
	model = updated.(CaptureModel)

	content := ansi.Strip(model.View().Content)
	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("view lines = %d, want 2: %q", len(lines), content)
	}
	if !strings.HasPrefix(lines[0], " gsd  ") {
		t.Errorf("input line = %q, want padded gsd badge", lines[0])
	}
	if width := lipgloss.Width(lines[0]); width != 40 {
		t.Errorf("input line width = %d, want 40", width)
	}
	if lines[1] != " "+captureFooter {
		t.Errorf("footer = %q, want %q", lines[1], " "+captureFooter)
	}
}

func assertInputBand(t *testing.T, model CaptureModel, background color.Color) {
	t.Helper()

	canvas := lipgloss.NewCanvas(model.width, 1).
		Compose(lipgloss.NewLayer(model.inputView()))
	badgeWidth := lipgloss.Width(" gsd ")
	for x := badgeWidth; x < model.width; x++ {
		assertColor(t, "input band", canvas.CellAt(x, 0).Style.Bg, background)
	}
}

func assertQuitCommand(t *testing.T, command tea.Cmd) {
	t.Helper()
	if command == nil {
		t.Fatal("quit command = nil")
	}
	message := command()
	if _, ok := message.(tea.QuitMsg); !ok {
		t.Fatalf("command message = %T, want tea.QuitMsg", message)
	}
}
