package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/pflag"
)

type interactiveColorCase struct {
	name            string
	environment     []string
	args            []string
	detected        colorprofile.Profile
	wantProfile     colorprofile.Profile
	wantDetection   bool
	wantEnvironment []string
}

func interactiveColorCases(command string) []interactiveColorCase {
	return []interactiveColorCase{
		{
			name:            "detected TrueColor",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{command},
			detected:        colorprofile.TrueColor,
			wantProfile:     colorprofile.TrueColor,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "detected ANSI256",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{command},
			detected:        colorprofile.ANSI256,
			wantProfile:     colorprofile.ANSI256,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "detected ANSI",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{command},
			detected:        colorprofile.ANSI,
			wantProfile:     colorprofile.ANSI,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "detected ASCII",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{command},
			detected:        colorprofile.ASCII,
			wantProfile:     colorprofile.ASCII,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "detected NoTTY",
			environment:     []string{"TERM=xterm", "KEEP=value"},
			args:            []string{command},
			detected:        colorprofile.NoTTY,
			wantProfile:     colorprofile.NoTTY,
			wantDetection:   true,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "NO_COLOR disables color",
			environment:     []string{"NO_COLOR=1", "TERM=xterm-256color", "KEEP=value"},
			args:            []string{command},
			wantProfile:     colorprofile.NoTTY,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "explicit auto still honors NO_COLOR",
			environment:     []string{"NO_COLOR=1", "TERM=xterm-256color", "KEEP=value"},
			args:            []string{command, "--color", "auto"},
			wantProfile:     colorprofile.NoTTY,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
		{
			name:            "explicit always overrides NO_COLOR",
			environment:     []string{"NO_COLOR=1", "TERM=xterm", "KEEP=value"},
			args:            []string{command, "--color", "always"},
			wantProfile:     colorprofile.TrueColor,
			wantEnvironment: []string{"TERM=xterm", "KEEP=value"},
		},
		{
			name:            "explicit never disables color",
			environment:     []string{"TERM=xterm-256color", "KEEP=value"},
			args:            []string{command, "--color=never"},
			wantProfile:     colorprofile.NoTTY,
			wantEnvironment: []string{"TERM=xterm-256color", "KEEP=value"},
		},
	}
}

func TestErrorStreamStaysUnstyledAndEscaped(t *testing.T) {
	t.Parallel()

	dependencies := render.PresentationDependencies{
		Environment:      func() []string { return []string{"TERM=xterm"} },
		IsTerminalWriter: func(io.Writer) bool { return true },
		DetectProfile: func(io.Writer, []string) colorprofile.Profile {
			return colorprofile.TrueColor
		},
		HasDarkBackground: func(io.Reader, io.Writer) bool {
			t.Fatal("background queried for an error stream")
			return true
		},
		Now: time.Now,
	}
	factory := func(
		context.Context,
		string,
		bool,
		*pflag.FlagSet,
	) (applications, io.Closer, error) {
		return applications{
			tasks: &fakeApplication{inboxError: errors.New("tag already exists: Evil\x1b]8;;https://example.com\a")},
		}, closeRecorder{close: func() {}}, nil
	}
	root := newRootCommandWithRuntimeDependencies(factory, nil, time.UTC, dependencies)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	if exitCode := execute(root, []string{"inbox"}); exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.String() != "" || stderr.String() != "gsd: tag already exists: Evil\\x1b]8;;https://example.com\\a\n" {
		t.Errorf("stdout/stderr = %q/%q, want escaped plain stderr error", stdout.String(), stderr.String())
	}
}

func TestColorFlagValidationAndForcedJSONCleanliness(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"inbox", "--color"}, {"inbox", "--color=sometimes"}, {"inbox", "--color="}} {
		result := runCommand(t, &fakeApplication{}, args...)
		if result.exitCode != 2 || result.opens != 0 || result.stdout != "" || result.stderr == "" {
			t.Errorf("%v result = %#v, want usage error without application open", args, result)
		}
	}

	tasks := []task.ViewTask{{Task: task.Task{ID: 1, Title: "capture", Status: "open"}}}
	human := runCommand(t, &fakeApplication{inboxResult: tasks}, "inbox", "--color=always")
	if human.exitCode != 0 || !strings.Contains(human.stdout, "\x1b[") {
		t.Errorf("forced human result = %#v, want ANSI", human)
	}
	jsonResult := runCommand(t, &fakeApplication{inboxResult: tasks}, "inbox", "--color=always", "--json")
	if jsonResult.exitCode != 0 || strings.Contains(jsonResult.stdout, "\x1b[") {
		t.Errorf("forced JSON result = %#v, want ANSI-free JSON", jsonResult)
	}
}
