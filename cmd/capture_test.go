package cmd

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/pflag"
)

func TestCaptureCommandPassesRuntimeDependenciesAndColorMode(t *testing.T) {
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
			args:            []string{"capture"},
			wantColor:       tui.ColorDetected,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "NO_COLOR disables color",
			environment:     []string{"NO_COLOR=1", "TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture"},
			wantColor:       tui.ColorDisabled,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "explicit always overrides environment",
			environment:     []string{"NO_COLOR=1", "TERM=dumb", "KEEP=value"},
			args:            []string{"capture", "--color", "always"},
			wantColor:       tui.ColorForced,
			wantEnvironment: []string{"TERM=dumb", "KEEP=value"},
		},
		{
			name:            "explicit never disables color",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture", "--color=never"},
			wantColor:       tui.ColorDisabled,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			application := &fakeApplication{}
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
				return applications{tasks: application}, closeRecorder{close: func() { closes++ }}, nil
			}

			dependencies := defaultPresentationDependencies()
			dependencies.environment = func() []string { return test.environment }
			inputChecked := false
			outputChecked := false
			dependencies.isTerminal = func(stream any) bool {
				switch stream {
				case input:
					inputChecked = true
					return true
				case &stdout:
					outputChecked = true
					return true
				default:
					return false
				}
			}

			runs := 0
			var gotApplication task.Application
			var gotOptions tui.ProgramOptions
			runCapture := func(
				_ context.Context,
				available task.Application,
				options tui.ProgramOptions,
			) error {
				runs++
				gotApplication = available
				gotOptions = options
				return nil
			}
			root := newRootCommandWithCaptureRunner(
				factory,
				nil,
				time.UTC,
				dependencies,
				runCapture,
			)
			root.SetIn(input)
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, test.args)

			if exitCode != 0 || stderr.String() != "" || stdout.String() != "" {
				t.Fatalf(
					"exit = %d, stdout = %q, stderr = %q; want successful silent adapter",
					exitCode,
					stdout.String(),
					stderr.String(),
				)
			}
			if opens != 1 || closes != 1 || runs != 1 {
				t.Fatalf("opens = %d, closes = %d, runs = %d; want one lifecycle", opens, closes, runs)
			}
			if !inputChecked || !outputChecked {
				t.Error("capture did not check both terminal input and terminal output")
			}
			if gotApplication != application {
				t.Fatal("capture application does not match factory task application")
			}
			if gotOptions.Input != input {
				t.Fatal("capture input does not match Cobra input")
			}
			if gotOptions.Output != &stdout {
				t.Fatal("capture output does not match Cobra output")
			}
			if gotOptions.Screen != tui.ScreenAlt {
				t.Errorf("capture screen = %v, want alternate screen", gotOptions.Screen)
			}
			if gotOptions.Color != test.wantColor {
				t.Errorf("capture color = %v, want %v", gotOptions.Color, test.wantColor)
			}
			if !reflect.DeepEqual(gotOptions.Environment, test.wantEnvironment) {
				t.Errorf("capture environment = %q, want %q", gotOptions.Environment, test.wantEnvironment)
			}
		})
	}
}

func TestCaptureCommandRejectsUnsupportedInvocationBeforeOpeningApplication(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		args     []string
		inputOK  bool
		outputOK bool
		message  string
	}{
		{
			name:    "JSON",
			args:    []string{"capture", "--json"},
			message: "--json is not supported by gsd capture; use gsd add TITLE for noninteractive capture",
		},
		{
			name:     "non-terminal input",
			args:     []string{"capture"},
			outputOK: true,
			message:  "gsd capture requires terminal input; use gsd add TITLE for noninteractive capture",
		},
		{
			name:    "non-terminal output",
			args:    []string{"capture"},
			inputOK: true,
			message: "gsd capture requires terminal output; use gsd add TITLE for noninteractive capture",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			input := strings.NewReader("")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			opens := 0
			runs := 0
			factory := func(
				context.Context,
				string,
				bool,
				*pflag.FlagSet,
			) (applications, io.Closer, error) {
				opens++
				return applications{tasks: &fakeApplication{}}, closeRecorder{close: func() {}}, nil
			}
			dependencies := defaultPresentationDependencies()
			dependencies.isTerminal = func(stream any) bool {
				switch stream {
				case input:
					return test.inputOK
				case &stdout:
					return test.outputOK
				default:
					return false
				}
			}
			root := newRootCommandWithCaptureRunner(
				factory,
				nil,
				time.UTC,
				dependencies,
				func(context.Context, task.Application, tui.ProgramOptions) error {
					runs++
					return nil
				},
			)
			root.SetIn(input)
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, test.args)

			if exitCode != 2 {
				t.Errorf("exit = %d, want 2", exitCode)
			}
			if stdout.String() != "" {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if stderr.String() != "Error: "+test.message+"\n" {
				t.Errorf("stderr = %q, want exact usage message %q", stderr.String(), test.message)
			}
			if opens != 0 || runs != 0 {
				t.Errorf("opens/runs = %d/%d, want 0/0", opens, runs)
			}
		})
	}
}
