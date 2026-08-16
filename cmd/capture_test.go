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

	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/pflag"
)

func TestCaptureCommandPassesRuntimeDependenciesAndColorCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		environment     []string
		args            []string
		detected        colorprofile.Profile
		wantProfile     colorprofile.Profile
		wantDetection   bool
		wantEnvironment []string
	}{
		{
			name:            "detected TrueColor",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture"},
			detected:        colorprofile.TrueColor,
			wantProfile:     colorprofile.TrueColor,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "detected ANSI256",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture"},
			detected:        colorprofile.ANSI256,
			wantProfile:     colorprofile.ANSI256,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "detected ANSI",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{"capture"},
			detected:        colorprofile.ANSI,
			wantProfile:     colorprofile.ANSI,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "detected ASCII",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{"capture"},
			detected:        colorprofile.ASCII,
			wantProfile:     colorprofile.ASCII,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "detected NoTTY",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{"capture"},
			detected:        colorprofile.NoTTY,
			wantProfile:     colorprofile.NoTTY,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "NO_COLOR disables color",
			environment:     []string{"NO_COLOR=1", "TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture"},
			wantProfile:     colorprofile.NoTTY,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "TERM dumb disables color",
			environment:     []string{"TERM=dumb", "KEEP=value"},
			args:            []string{"capture"},
			wantProfile:     colorprofile.NoTTY,
			wantEnvironment: []string{"TERM=dumb", "KEEP=value"},
		},
		{
			name:            "explicit always overrides environment",
			environment:     []string{"NO_COLOR=1", "TERM=dumb", "KEEP=value"},
			args:            []string{"capture", "--color", "always"},
			wantProfile:     colorprofile.TrueColor,
			wantEnvironment: []string{"TERM=dumb", "KEEP=value"},
		},
		{
			name:            "explicit never disables color",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{"capture", "--color=never"},
			wantProfile:     colorprofile.NoTTY,
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
			dependencies.isTerminalReader = func(reader io.Reader) bool {
				if reader == input {
					inputChecked = true
				}
				return reader == input
			}
			dependencies.isTerminalWriter = func(writer io.Writer) bool {
				if writer == &stdout {
					outputChecked = true
				}
				return writer == &stdout
			}
			detections := 0
			dependencies.detectProfile = func(io.Writer, []string) colorprofile.Profile {
				detections++
				return test.detected
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
			root := newRootCommandWithRunners(
				factory,
				nil,
				time.UTC,
				dependencies,
				runners{capture: runCapture},
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
			if gotOptions.Profile != test.wantProfile {
				t.Errorf("capture profile = %v, want %v", gotOptions.Profile, test.wantProfile)
			}
			if !gotOptions.Terminal {
				t.Error("capture terminal destination = false, want true")
			}
			wantColorEnabled := test.wantProfile >= colorprofile.ANSI
			if got := gotOptions.ColorEnabled(); got != wantColorEnabled {
				t.Errorf("capture color enabled = %t, want %t", got, wantColorEnabled)
			}
			if got := detections > 0; got != test.wantDetection {
				t.Errorf("capture profile detection = %t, want %t", got, test.wantDetection)
			}
			if detections > 1 {
				t.Errorf("capture profile detections = %d, want at most 1", detections)
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
			name:    "forced color with non-terminal output",
			args:    []string{"capture", "--color=always"},
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
			dependencies.isTerminalReader = func(io.Reader) bool { return test.inputOK }
			dependencies.isTerminalWriter = func(io.Writer) bool { return test.outputOK }
			root := newRootCommandWithRunners(
				factory,
				nil,
				time.UTC,
				dependencies,
				runners{capture: func(context.Context, task.Application, tui.ProgramOptions) error {
					runs++
					return nil
				}},
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

func TestCaptureCommandMapsRunnerErrorsToExitOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "application error",
			err:     apperr.New(apperr.Conflict, "task already exists", nil),
			message: "task already exists",
		},
		{
			name:    "uncoded program error",
			err:     errors.New("terminal torn down"),
			message: "terminal torn down",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

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
				return applications{tasks: &fakeApplication{}}, closeRecorder{close: func() { closes++ }}, nil
			}
			dependencies := defaultPresentationDependencies()
			dependencies.environment = func() []string { return []string{"TERM=xterm"} }
			dependencies.isTerminalReader = func(io.Reader) bool { return true }
			dependencies.isTerminalWriter = func(io.Writer) bool { return true }
			root := newRootCommandWithRunners(
				factory,
				nil,
				time.UTC,
				dependencies,
				runners{capture: func(context.Context, task.Application, tui.ProgramOptions) error {
					return test.err
				}},
			)
			root.SetIn(input)
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			exitCode := execute(root, []string{"capture"})

			if exitCode != 1 {
				t.Errorf("exit = %d, want 1", exitCode)
			}
			if stdout.String() != "" {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if stderr.String() != "Error: "+test.message+"\n" {
				t.Errorf("stderr = %q, want standard error line %q", stderr.String(), test.message)
			}
			if opens != 1 || closes != 1 {
				t.Errorf("opens/closes = %d/%d, want one lifecycle", opens, closes)
			}
		})
	}
}
