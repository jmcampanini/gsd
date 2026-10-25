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
		explicit     bool
		noColor      string
		terminal     bool
		terminalName string
		want         colorDecision
	}{
		{name: "default terminal", mode: ColorAuto, terminal: true, terminalName: "xterm-256color", want: colorDetected},
		{name: "default pipe", mode: ColorAuto, terminalName: "xterm-256color", want: colorDisabled},
		{name: "default dumb terminal", mode: ColorAuto, terminal: true, terminalName: "dumb", want: colorDisabled},
		{name: "nonempty NO_COLOR", mode: ColorAuto, noColor: "0", terminal: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit always beats everything", mode: ColorAlways, explicit: true, noColor: "1", terminalName: "dumb", want: colorForced},
		{name: "explicit never", mode: ColorNever, explicit: true, terminal: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit auto beats NO_COLOR", mode: ColorAuto, explicit: true, noColor: "1", terminal: true, terminalName: "xterm", want: colorDetected},
		{name: "explicit auto still respects pipe", mode: ColorAuto, explicit: true, terminalName: "xterm", want: colorDisabled},
		{name: "explicit auto still respects dumb", mode: ColorAuto, explicit: true, terminal: true, terminalName: "dumb", want: colorDisabled},
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

	mode := ColorAuto
	var detectedEnvironment []string
	available := Presentation{
		Mode: &mode,
		Dependencies: PresentationDependencies{
			Environment: func() []string {
				return []string{
					"TERM=xterm-256color",
					"NO_COLOR=1",
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
	resolution := available.Resolve(io.Discard, true)
	if resolution.Profile != colorprofile.ANSI256 || !resolution.Terminal {
		t.Fatalf("profile/terminal = %s/%t, want ANSI256/true", resolution.Profile, resolution.Terminal)
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
		mode        ColorMode
		explicit    bool
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
			explicit:   true,
			detected:   colorprofile.NoTTY,
			wantANSI:   true,
			wantAccent: frappeGreenSGR,
		},
		{name: "never terminal", mode: ColorNever, explicit: true, terminal: true, detected: colorprofile.TrueColor},
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
			human := available.Output(strings.NewReader(""), &output, test.explicit)
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
