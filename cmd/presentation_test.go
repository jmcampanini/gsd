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
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestResolveColorPrecedenceAndTerminalBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         colorMode
		explicit     bool
		noColor      string
		terminal     bool
		terminalName string
		want         colorDecision
	}{
		{name: "default terminal", mode: colorAuto, terminal: true, terminalName: "xterm-256color", want: colorDetected},
		{name: "default pipe", mode: colorAuto, terminalName: "xterm-256color", want: colorDisabled},
		{name: "default dumb terminal", mode: colorAuto, terminal: true, terminalName: "dumb", want: colorDisabled},
		{name: "nonempty NO_COLOR", mode: colorAuto, noColor: "0", terminal: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit always beats everything", mode: colorAlways, explicit: true, noColor: "1", terminalName: "dumb", want: colorForced},
		{name: "explicit never", mode: colorNever, explicit: true, terminal: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit auto beats NO_COLOR", mode: colorAuto, explicit: true, noColor: "1", terminal: true, terminalName: "xterm", want: colorDetected},
		{name: "explicit auto still respects pipe", mode: colorAuto, explicit: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit auto still respects dumb", mode: colorAuto, explicit: true, terminal: true, terminalName: "dumb", want: colorDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := resolveColor(test.mode, test.explicit, test.noColor, test.terminal, test.terminalName)
			if got != test.want {
				t.Errorf("resolveColor() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestPresentationProfileUsesScrubbedEnvironment(t *testing.T) {
	t.Parallel()

	mode := colorAuto
	var detectedEnvironment []string
	available := presentation{
		mode: &mode,
		dependencies: presentationDependencies{
			environment: func() []string {
				return []string{
					"TERM=xterm-256color",
					"NO_COLOR=1",
					"FORCE_COLOR=1",
					"CLICOLOR=1",
					"CLICOLOR_FORCE=1",
					"COLORTERM=truecolor",
				}
			},
			isTerminalWriter: func(io.Writer) bool { return true },
			detectProfile: func(_ io.Writer, environment []string) colorprofile.Profile {
				detectedEnvironment = append([]string(nil), environment...)
				return colorprofile.ANSI256
			},
		},
		location: time.UTC,
	}
	resolution := available.resolve(io.Discard, true)
	if resolution.profile != colorprofile.ANSI256 || !resolution.terminal {
		t.Fatalf("profile/terminal = %s/%t, want ANSI256/true", resolution.profile, resolution.terminal)
	}
	want := []string{"TERM=xterm-256color", "COLORTERM=truecolor"}
	if !reflect.DeepEqual(detectedEnvironment, want) {
		t.Errorf("detected environment = %#v, want %#v", detectedEnvironment, want)
	}
}

func TestPresentationQueriesBackgroundOnceOnlyForStyledTerminalStdout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        colorMode
		explicit    bool
		terminal    bool
		detected    colorprofile.Profile
		wantQueries int
		wantANSI    bool
		wantAccent  string
	}{
		{name: "detected terminal", mode: colorAuto, terminal: true, detected: colorprofile.ANSI, wantQueries: 1, wantANSI: true},
		{
			name:        "detected truecolor terminal uses queried light background",
			mode:        colorAuto,
			terminal:    true,
			detected:    colorprofile.TrueColor,
			wantQueries: 1,
			wantANSI:    true,
			wantAccent:  latteGreenSGR,
		},
		{name: "detected ASCII terminal", mode: colorAuto, terminal: true, detected: colorprofile.ASCII, wantANSI: true},
		{name: "detected no-style terminal", mode: colorAuto, terminal: true, detected: colorprofile.NoTTY},
		{
			name:       "forced pipe defaults to dark accents",
			mode:       colorAlways,
			explicit:   true,
			detected:   colorprofile.NoTTY,
			wantANSI:   true,
			wantAccent: frappeGreenSGR,
		},
		{name: "never terminal", mode: colorNever, explicit: true, terminal: true, detected: colorprofile.TrueColor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mode := test.mode
			queries := 0
			available := presentation{
				mode: &mode,
				dependencies: presentationDependencies{
					environment:      func() []string { return []string{"TERM=xterm"} },
					isTerminalWriter: func(io.Writer) bool { return test.terminal },
					detectProfile: func(io.Writer, []string) colorprofile.Profile {
						return test.detected
					},
					hasDarkBackground: func(io.Reader, io.Writer) bool {
						queries++
						return false
					},
					now: func() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) },
				},
				location: time.UTC,
			}
			root := &cobra.Command{Use: "test"}
			root.PersistentFlags().Var(colorValue{mode: &mode}, "color", "color")
			if test.explicit {
				if err := root.PersistentFlags().Set("color", string(test.mode)); err != nil {
					t.Fatalf("set color: %v", err)
				}
			}
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetIn(strings.NewReader(""))
			human := available.output(root)
			if err := human.writeTaskMutation(verbDone, task.Task{ID: 7, Title: "capture"}); err != nil {
				t.Fatalf("write mutation: %v", err)
			}
			if queries != test.wantQueries {
				t.Errorf("background queries = %d, want %d", queries, test.wantQueries)
			}
			if strings.Contains(output.String(), "\x1b[") != test.wantANSI {
				t.Errorf("output = %q, ANSI presence want %t", output.String(), test.wantANSI)
			}
			if test.wantAccent != "" && !strings.Contains(output.String(), test.wantAccent) {
				t.Errorf("output = %q, want accent %s", output.String(), test.wantAccent)
			}
		})
	}
}

func TestErrorStreamStaysUnstyledAndEscaped(t *testing.T) {
	t.Parallel()

	dependencies := presentationDependencies{
		environment:      func() []string { return []string{"TERM=xterm"} },
		isTerminalWriter: func(io.Writer) bool { return true },
		detectProfile: func(io.Writer, []string) colorprofile.Profile {
			return colorprofile.TrueColor
		},
		hasDarkBackground: func(io.Reader, io.Writer) bool {
			t.Fatal("background queried for an error stream")
			return true
		},
		now: time.Now,
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
	if stdout.String() != "" || stderr.String() != "Error: tag already exists: Evil\\x1b]8;;https://example.com\\a\n" {
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
