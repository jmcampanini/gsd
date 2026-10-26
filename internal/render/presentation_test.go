package render

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestResolveColorPrecedenceAndTerminalBranches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         ColorMode
		noColor      string
		terminal     bool
		terminalName string
		want         colorDecision
	}{
		{name: "auto terminal", mode: ColorAuto, terminal: true, terminalName: "xterm-256color", want: colorDetected},
		{name: "auto pipe", mode: ColorAuto, terminalName: "xterm-256color", want: colorDisabled},
		{name: "auto dumb terminal", mode: ColorAuto, terminal: true, terminalName: "dumb", want: colorDisabled},
		{name: "auto honors nonempty NO_COLOR", mode: ColorAuto, noColor: "0", terminal: true, terminalName: "xterm", want: colorDisabled},
		{name: "auto ignores empty NO_COLOR", mode: ColorAuto, noColor: "", terminal: true, terminalName: "xterm", want: colorDetected},
		{name: "always beats everything", mode: ColorAlways, noColor: "1", terminalName: "dumb", want: colorForced},
		{name: "never beats a terminal", mode: ColorNever, terminal: true, terminalName: "xterm", want: colorDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := resolveColor(test.mode, test.noColor, test.terminal, test.terminalName)
			if got != test.want {
				t.Errorf("resolveColor() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestPresentationProfileUsesScrubbedEnvironment(t *testing.T) {
	t.Parallel()

	mode := ColorAuto
	var detectedEnvironment []string
	available := Presentation{
		Mode: &mode,
		Dependencies: PresentationDependencies{
			Environment: func() []string {
				return []string{
					"TERM=xterm-256color",
					"NO_COLOR=",
					"FORCE_COLOR=1",
					"CLICOLOR=1",
					"CLICOLOR_FORCE=1",
					"COLORTERM=truecolor",
				}
			},
			IsTerminalWriter: func(io.Writer) bool { return true },
			DetectProfile: func(_ io.Writer, environment []string) colorprofile.Profile {
				detectedEnvironment = append([]string(nil), environment...)
				return colorprofile.ANSI256
			},
		},
		Location: time.UTC,
	}
	resolution := available.Resolve(io.Discard)
	if resolution.Profile != colorprofile.ANSI256 || !resolution.Terminal {
		t.Fatalf("profile/terminal = %s/%t, want ANSI256/true", resolution.Profile, resolution.Terminal)
	}
	want := []string{"TERM=xterm-256color", "COLORTERM=truecolor"}
	if !reflect.DeepEqual(detectedEnvironment, want) {
		t.Errorf("detected environment = %#v, want %#v", detectedEnvironment, want)
	}
}

func TestResolveObservesCursorControlFromTERMAlone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		environment       []string
		terminal          bool
		detected          colorprofile.Profile
		wantCursorControl bool
	}{
		{name: "named terminal", environment: []string{"TERM=xterm-256color"}, terminal: true, detected: colorprofile.TrueColor, wantCursorControl: true},
		{name: "dumb terminal", environment: []string{"TERM=dumb"}, terminal: true, detected: colorprofile.TrueColor},
		{name: "unset TERM", environment: []string{"KEEP=value"}, terminal: true, detected: colorprofile.TrueColor},
		{name: "empty TERM", environment: []string{"TERM="}, terminal: true, detected: colorprofile.TrueColor},
		{name: "named terminal on a pipe", environment: []string{"TERM=xterm"}, detected: colorprofile.NoTTY, wantCursorControl: true},
		{name: "named terminal without color", environment: []string{"TERM=xterm", "NO_COLOR=1"}, terminal: true, detected: colorprofile.NoTTY, wantCursorControl: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mode := ColorAuto
			available := Presentation{
				Mode: &mode,
				Dependencies: PresentationDependencies{
					Environment:      func() []string { return test.environment },
					IsTerminalWriter: func(io.Writer) bool { return test.terminal },
					DetectProfile: func(io.Writer, []string) colorprofile.Profile {
						return test.detected
					},
				},
				Location: time.UTC,
			}
			resolution := available.Resolve(io.Discard)
			if resolution.CursorControl != test.wantCursorControl {
				t.Errorf("cursor control = %t, want %t", resolution.CursorControl, test.wantCursorControl)
			}
			if resolution.Terminal != test.terminal {
				t.Errorf("terminal = %t, want %t (cursor control must not change it)", resolution.Terminal, test.terminal)
			}
		})
	}
}

func TestPresentationQueriesBackgroundOnceOnlyForStyledTerminalStdout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mode        ColorMode
		terminal    bool
		detected    colorprofile.Profile
		wantQueries int
		wantANSI    bool
		wantAccent  string
	}{
		{name: "detected terminal", mode: ColorAuto, terminal: true, detected: colorprofile.ANSI, wantQueries: 1, wantANSI: true},
		{
			name:        "detected truecolor terminal uses queried light background",
			mode:        ColorAuto,
			terminal:    true,
			detected:    colorprofile.TrueColor,
			wantQueries: 1,
			wantANSI:    true,
			wantAccent:  latteGreenSGR,
		},
		{name: "detected ASCII terminal", mode: ColorAuto, terminal: true, detected: colorprofile.ASCII, wantANSI: true},
		{name: "detected no-style terminal", mode: ColorAuto, terminal: true, detected: colorprofile.NoTTY},
		{
			name:       "forced pipe defaults to dark accents",
			mode:       ColorAlways,
			detected:   colorprofile.NoTTY,
			wantANSI:   true,
			wantAccent: frappeGreenSGR,
		},
		{name: "never terminal", mode: ColorNever, terminal: true, detected: colorprofile.TrueColor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mode := test.mode
			queries := 0
			available := Presentation{
				Mode: &mode,
				Dependencies: PresentationDependencies{
					Environment:      func() []string { return []string{"TERM=xterm"} },
					IsTerminalWriter: func(io.Writer) bool { return test.terminal },
					DetectProfile: func(io.Writer, []string) colorprofile.Profile {
						return test.detected
					},
					HasDarkBackground: func(io.Reader, io.Writer) bool {
						queries++
						return false
					},
					Now: func() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) },
				},
				Location: time.UTC,
			}
			var output bytes.Buffer
			human := available.Output(strings.NewReader(""), &output)
			if err := human.writeTaskMutation(VerbDone, task.Task{ID: 7, Title: "capture"}); err != nil {
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
