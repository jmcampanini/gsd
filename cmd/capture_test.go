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

func TestCaptureCommandPassesRuntimeDependencies(t *testing.T) {
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
	dependencies.environment = func() []string {
		return []string{"NO_COLOR=1", "TERM=dumb", "KEEP=value"}
	}
	dependencies.isTerminal = func(io.Writer) bool { return false }

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

	exitCode := execute(root, []string{"capture", "--color", "always"})

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
	if gotOptions.Color != tui.ColorForced {
		t.Errorf("capture color = %v, want forced color", gotOptions.Color)
	}
	if !reflect.DeepEqual(gotOptions.Environment, []string{"TERM=dumb", "KEEP=value"}) {
		t.Errorf("capture environment = %q, want scrubbed color environment", gotOptions.Environment)
	}
}
