package tui

import (
	"context"
	"errors"
	"image/color"
	"reflect"
	"strings"
	"testing"
	"time"

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

type blockingCaptureApplication struct {
	task.Application
	calls   chan context.Context
	release chan struct{}
	err     error
}

func (a *blockingCaptureApplication) Add(
	ctx context.Context,
	_ task.AddFields,
) (task.Task, error) {
	a.calls <- ctx
	<-a.release
	return task.Task{}, a.err
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
	if application.ctx == ctx {
		t.Fatal("Add context should be a child of the capture context")
	}
	if got := application.ctx.Value(contextKey{}); got != "capture" {
		t.Fatalf("Add context value = %v, want inherited capture value", got)
	}
	wantFields := task.AddFields{Title: title}
	if !reflect.DeepEqual(application.fields, wantFields) {
		t.Fatalf("Add fields = %#v, want %#v", application.fields, wantFields)
	}

	wantFooter := " " + captureAddingStatus
	if got := footerText(model); got != wantFooter {
		t.Fatalf("submitting footer = %q, want %q", got, wantFooter)
	}
	if model.View().Cursor != nil {
		t.Fatal("submitting cursor should be hidden")
	}

	completed, quitCommand := model.Update(result)
	model = completed.(CaptureModel)
	_, command := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if command != nil || application.calls != 1 {
		t.Fatal("Enter while the successful quit is pending should not start another Add")
	}
	assertQuitCommand(t, quitCommand)
	if model.Err() != nil {
		t.Fatalf("capture error = %v, want nil", model.Err())
	}
	select {
	case <-application.ctx.Done():
	default:
		t.Fatal("Add child context resources were not released on result")
	}
}

func TestCaptureSubmittingCancelWaitsForAdd(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "escape", key: tea.KeyPressMsg{Code: tea.KeyEscape}},
		{name: "control-c", key: tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := &blockingCaptureApplication{
				calls:   make(chan context.Context, 1),
				release: make(chan struct{}),
				err:     context.Canceled,
			}
			model := NewCaptureModel(context.Background(), application, false)
			model.input.SetValue("Call plumber")
			updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model = updated.(CaptureModel)
			results := make(chan tea.Msg, 1)
			go func() { results <- addCommand() }()

			addContext := receiveContext(t, application.calls)
			updated, command := model.Update(test.key)
			model = updated.(CaptureModel)
			if command != nil {
				t.Fatal("cancel while submitting should wait, not quit")
			}
			wantFooter := " " + captureCancelStatus
			if got := footerText(model); got != wantFooter {
				t.Fatalf("cancellation footer = %q, want %q", got, wantFooter)
			}
			select {
			case <-addContext.Done():
			case <-time.After(time.Second):
				t.Fatal("Add context was not canceled")
			}
			select {
			case <-results:
				t.Fatal("Add returned before the blocking application was released")
			default:
			}

			updated, duplicateCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model = updated.(CaptureModel)
			if duplicateCommand != nil {
				t.Fatal("Enter after cancellation should not start another Add")
			}
			close(application.release)
			result := receiveMessage(t, results)
			completed, quitCommand := model.Update(result)
			model = completed.(CaptureModel)
			assertQuitCommand(t, quitCommand)
			if model.Err() != nil {
				t.Fatalf("requested cancellation error = %v, want nil", model.Err())
			}
			if len(application.calls) != 0 {
				t.Fatal("capture started a duplicate Add")
			}
		})
	}
}

func TestCaptureAddSuccessWinsCancellationRace(t *testing.T) {
	application := &blockingCaptureApplication{
		calls:   make(chan context.Context, 1),
		release: make(chan struct{}),
	}
	model := NewCaptureModel(context.Background(), application, false)
	model.input.SetValue("Call plumber")
	updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(CaptureModel)
	results := make(chan tea.Msg, 1)
	go func() { results <- addCommand() }()
	addContext := receiveContext(t, application.calls)

	updated, command := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(CaptureModel)
	if command != nil {
		t.Fatal("cancel while submitting should wait, not quit")
	}
	select {
	case <-addContext.Done():
	case <-time.After(time.Second):
		t.Fatal("Add context was not canceled")
	}
	close(application.release)

	completed, quitCommand := model.Update(receiveMessage(t, results))
	model = completed.(CaptureModel)
	assertQuitCommand(t, quitCommand)
	if model.Err() != nil {
		t.Fatalf("successful Add after cancellation = %v, want nil", model.Err())
	}
}

func TestCaptureAddFailureWinsCancellationRequest(t *testing.T) {
	failure := errors.New("save failed")
	model := NewCaptureModel(
		context.Background(),
		&captureApplication{err: failure},
		false,
	)
	model.input.SetValue("Call plumber")
	updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = updated.(CaptureModel)
	result := addCommand()

	updated, command := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	model = updated.(CaptureModel)
	if command != nil {
		t.Fatal("cancel while submitting should wait, not quit")
	}

	updated, command = model.Update(result)
	model = updated.(CaptureModel)
	if command != nil {
		t.Fatal("application failure should remain visible after cancellation request")
	}
	if !errors.Is(model.Err(), failure) {
		t.Fatalf("capture error = %v, want %v", model.Err(), failure)
	}
}

func TestCaptureAddFailurePersistsUntilKeyDismissal(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "application failure", err: errors.New("save failed")},
		{name: "unrequested cancellation", err: context.Canceled},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			application := &captureApplication{err: test.err}
			model := NewCaptureModel(context.Background(), application, false)
			model.input.SetValue("Call plumber")
			updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model = updated.(CaptureModel)

			updated, command := model.Update(addCommand())
			model = updated.(CaptureModel)
			if command != nil {
				t.Fatal("failed Add should remain visible until a key is pressed")
			}
			if !errors.Is(model.Err(), test.err) {
				t.Fatalf("capture error = %v, want %v", model.Err(), test.err)
			}
			if model.View().Cursor != nil {
				t.Fatal("error-state cursor should be hidden")
			}

			updated, quitCommand := model.Update(tea.KeyPressMsg{Text: "x", Code: 'x'})
			model = updated.(CaptureModel)
			assertQuitCommand(t, quitCommand)
			if !errors.Is(model.Err(), test.err) {
				t.Fatalf("dismissed capture error = %v, want retained %v", model.Err(), test.err)
			}
		})
	}
}

func TestCaptureErrorFooterIsSanitizedAndStyled(t *testing.T) {
	failure := errors.New("bad\nvalue\x1b]8;;https://example.com\a")
	want := " Error: bad\\nvalue\\x1b]8;;https://example.com\\a"
	tests := []struct {
		name  string
		color bool
	}{
		{name: "styled", color: true},
		{name: "plain"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewCaptureModel(
				context.Background(),
				&captureApplication{err: failure},
				test.color,
			)
			model.input.SetValue("Call plumber")
			updated, addCommand := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			model = updated.(CaptureModel)
			updated, _ = model.Update(addCommand())
			model = updated.(CaptureModel)

			content := ansi.Strip(model.View().Content)
			lines := strings.Split(content, "\n")
			if len(lines) != 2 || !strings.Contains(lines[0], "Call plumber") {
				t.Fatalf("error view did not preserve input row: %q", content)
			}
			if lines[1] != want {
				t.Fatalf("error footer = %q, want %q", lines[1], want)
			}
			if test.color {
				assertColor(t, "error footer", model.errorStyle.GetForeground(), model.theme.Red)
				if !strings.Contains(model.View().Content, "\x1b[") {
					t.Fatal("styled error footer has no ANSI styling")
				}
			} else {
				if !reflect.DeepEqual(model.errorStyle.GetForeground(), lipgloss.NoColor{}) {
					t.Errorf("plain error foreground = %v, want no color", model.errorStyle.GetForeground())
				}
				if strings.Contains(model.View().Content, "\x1b[") {
					t.Fatal("plain error footer contains ANSI styling")
				}
			}
		})
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
	model.setTheme(theme)
	cursor := model.View().Cursor
	if cursor == nil {
		t.Fatal("themed cursor = nil, want visible cursor")
	}
	assertColor(t, "themed cursor", cursor.Color, theme.Cursor)
}

func TestCaptureCursorFollowsDisplayWidth(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "wide rune", value: "界"},
		{name: "combining grapheme", value: "e\u0301"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewCaptureModel(context.Background(), &captureApplication{}, false)
			updated, _ := model.Update(tea.WindowSizeMsg{Width: 20, Height: 2})
			model = updated.(CaptureModel)
			model.input.SetValue(test.value)

			cursor := model.View().Cursor
			if cursor == nil {
				t.Fatal("capture cursor = nil, want visible cursor")
			}
			wantX := lipgloss.Width(model.input.Prompt) + lipgloss.Width(test.value)
			if cursor.X != wantX {
				t.Errorf("capture cursor X = %d, want %d", cursor.X, wantX)
			}
		})
	}
}

func TestCaptureCursorFollowsScrolledViewport(t *testing.T) {
	model := NewCaptureModel(context.Background(), &captureApplication{}, false)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 12, Height: 2})
	model = updated.(CaptureModel)
	model.input.SetValue("abcdefgh")

	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	model = updated.(CaptureModel)

	input := ansi.Strip(model.input.View())
	wantX := strings.Index(input, "h")
	if wantX < 0 {
		t.Fatalf("rendered input = %q, want h under cursor", input)
	}
	cursor := model.View().Cursor
	if cursor == nil {
		t.Fatal("capture cursor = nil, want visible cursor")
	}
	if cursor.X != wantX {
		t.Errorf("capture cursor X = %d, want visible h at %d", cursor.X, wantX)
	}
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

func footerText(model CaptureModel) string {
	lines := strings.Split(ansi.Strip(model.View().Content), "\n")
	return lines[len(lines)-1]
}

func receiveContext(t *testing.T, contexts <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-contexts:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Add to start")
		return nil
	}
}

func receiveMessage(t *testing.T, messages <-chan tea.Msg) tea.Msg {
	t.Helper()
	select {
	case msg := <-messages:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Add to return")
		return nil
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
