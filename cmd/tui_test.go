package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/jmcampanini/gsd/internal/tui/navigator"
	"github.com/spf13/pflag"
)

func TestTUICommandPassesApplicationsRuntimeOptionsAndLocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		environment     []string
		args            []string
		wantColor       tui.ColorMode
		wantEnvironment []string
	}{
		{
			name:            "automatic terminal color",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"tui"},
			wantColor:       tui.ColorDetected,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "NO_COLOR disables color",
			environment:     []string{"NO_COLOR=1", "TERM=xterm-256color", "KEEP=value"},
			args:            []string{"tui"},
			wantColor:       tui.ColorDisabled,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "explicit always overrides environment",
			environment:     []string{"NO_COLOR=1", "TERM=dumb", "KEEP=value"},
			args:            []string{"tui", "--color", "always"},
			wantColor:       tui.ColorForced,
			wantEnvironment: []string{"TERM=dumb", "KEEP=value"},
		},
		{
			name:            "explicit never disables color",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"tui", "--color=never"},
			wantColor:       tui.ColorDisabled,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tasks := &fakeApplication{}
			projects := &fakeProjectApplication{}
			areas := &fakeAreaApplication{}
			boards := &fakeBoardApplication{}
			logbook := &fakeLogbookApplication{}
			input := strings.NewReader("")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			opens := 0
			closes := 0
			factory := func(
				context.Context,
				string,
				bool,
				*pflag.FlagSet,
			) (applications, io.Closer, error) {
				opens++
				return applications{
					tasks: tasks, projects: projects, areas: areas, boards: boards, logbook: logbook,
				}, closeRecorder{close: func() { closes++ }}, nil
			}

			presentation := defaultPresentationDependencies()
			presentation.environment = func() []string { return test.environment }
			inputChecked := false
			outputChecked := false
			presentation.isTerminalReader = func(reader io.Reader) bool {
				inputChecked = reader == input
				return inputChecked
			}
			presentation.isTerminalWriter = func(writer io.Writer) bool {
				outputChecked = writer == &stdout
				return outputChecked
			}

			location := time.FixedZone("test", 2*60*60)
			runs := 0
			var gotDependencies navigator.Dependencies
			var gotOptions tui.ProgramOptions
			var gotLocation *time.Location
			runNavigator := func(
				_ context.Context,
				dependencies navigator.Dependencies,
				options tui.ProgramOptions,
				location *time.Location,
			) error {
				runs++
				gotDependencies = dependencies
				gotOptions = options
				gotLocation = location
				return nil
			}
			root := newRootCommandWithRunners(
				factory,
				nil,
				location,
				presentation,
				runners{navigator: runNavigator},
			)
			root.SetIn(input)
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, test.args)

			if exitCode != 0 || stdout.String() != "" || stderr.String() != "" {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q; want successful silent adapter", exitCode, stdout.String(), stderr.String())
			}
			if opens != 1 || closes != 1 || runs != 1 {
				t.Fatalf("opens = %d, closes = %d, runs = %d; want one lifecycle", opens, closes, runs)
			}
			if !inputChecked || !outputChecked {
				t.Error("tui did not check both terminal input and terminal output")
			}
			if gotDependencies.Tasks != tasks || gotDependencies.Projects != projects ||
				gotDependencies.Areas != areas || gotDependencies.Boards != boards ||
				gotDependencies.Logbook != logbook {
				t.Error("navigator dependencies do not match the five factory applications")
			}
			if gotOptions.Input != input || gotOptions.Output != &stdout {
				t.Error("navigator streams do not match Cobra streams")
			}
			if gotOptions.Screen != tui.ScreenAlt {
				t.Errorf("navigator screen = %v, want alternate screen", gotOptions.Screen)
			}
			if gotOptions.Color != test.wantColor {
				t.Errorf("navigator color = %v, want %v", gotOptions.Color, test.wantColor)
			}
			if !reflect.DeepEqual(gotOptions.Environment, test.wantEnvironment) {
				t.Errorf("navigator environment = %q, want %q", gotOptions.Environment, test.wantEnvironment)
			}
			if gotLocation != location {
				t.Error("navigator location does not match the root location")
			}
		})
	}
}

func TestTUICommandRejectsNoninteractiveInvocationBeforeOpeningApplication(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		inputOK  bool
		outputOK bool
		marker   string
	}{
		{name: "positional argument", args: []string{"tui", "extra"}, inputOK: true, outputOK: true, marker: "positional arguments"},
		{name: "JSON", args: []string{"tui", "--json"}, inputOK: true, outputOK: true, marker: "--json"},
		{name: "non-terminal input", args: []string{"tui"}, outputOK: true, marker: "terminal input"},
		{name: "non-terminal output", args: []string{"tui"}, inputOK: true, marker: "terminal output"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			opens := 0
			runs := 0
			factory := func(context.Context, string, bool, *pflag.FlagSet) (applications, io.Closer, error) {
				opens++
				return applications{}, closeRecorder{close: func() {}}, nil
			}
			presentation := defaultPresentationDependencies()
			presentation.isTerminalReader = func(io.Reader) bool { return test.inputOK }
			presentation.isTerminalWriter = func(io.Writer) bool { return test.outputOK }
			root := newRootCommandWithRunners(
				factory,
				nil,
				time.UTC,
				presentation,
				runners{navigator: func(context.Context, navigator.Dependencies, tui.ProgramOptions, *time.Location) error {
					runs++
					return nil
				}},
			)
			root.SetIn(strings.NewReader(""))
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, test.args)

			if exitCode != 2 || stdout.String() != "" {
				t.Errorf("exit/stdout = %d/%q, want usage exit 2 and empty stdout", exitCode, stdout.String())
			}
			if !strings.Contains(stderr.String(), test.marker) ||
				!strings.HasSuffix(stderr.String(), "use the gsd CLI for noninteractive access\n") {
				t.Errorf("stderr = %q, want %q and required CLI guidance", stderr.String(), test.marker)
			}
			if opens != 0 || runs != 0 {
				t.Errorf("opens/runs = %d/%d, want rejection before lifecycle", opens, runs)
			}
		})
	}
}

func TestTUICommandMapsRunnerErrorsToExitOne(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "coded application error", err: apperr.New(apperr.Conflict, "view unavailable", nil)},
		{name: "uncoded program error", err: errors.New("terminal torn down")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			closes := 0
			factory := func(context.Context, string, bool, *pflag.FlagSet) (applications, io.Closer, error) {
				return applications{}, closeRecorder{close: func() { closes++ }}, nil
			}
			presentation := defaultPresentationDependencies()
			presentation.isTerminalReader = func(io.Reader) bool { return true }
			presentation.isTerminalWriter = func(io.Writer) bool { return true }
			root := newRootCommandWithRunners(
				factory,
				nil,
				time.UTC,
				presentation,
				runners{navigator: func(context.Context, navigator.Dependencies, tui.ProgramOptions, *time.Location) error {
					return test.err
				}},
			)
			root.SetIn(strings.NewReader(""))
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, []string{"tui"})

			if exitCode != 1 || stdout.String() != "" || closes != 1 {
				t.Errorf("exit/stdout/closes = %d/%q/%d, want stderr-only exit 1 and close", exitCode, stdout.String(), closes)
			}
			if stderr.String() != "Error: "+test.err.Error()+"\n" {
				t.Errorf("stderr = %q, want standard runner diagnostic", stderr.String())
			}
		})
	}
}
