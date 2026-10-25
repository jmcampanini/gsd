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
		Mode         ColorMode
		explicit     bool
		noColor      string
		Terminal     bool
		terminalName string
		want         ColorDecision
	}{
		{name: "default terminal", Mode: ColorAuto, Terminal: true, terminalName: "xterm-256color", want: ColorDetected},
		{name: "default pipe", Mode: ColorAuto, terminalName: "xterm-256color", want: ColorDisabled},
		{name: "default dumb terminal", Mode: ColorAuto, Terminal: true, terminalName: "dumb", want: ColorDisabled},
		{name: "nonempty NO_COLOR", Mode: ColorAuto, noColor: "0", Terminal: true, terminalName: "xterm", want: ColorDisabled},
		{name: "explicit always beats everything", Mode: ColorAlways, explicit: true, noColor: "1", terminalName: "dumb", want: ColorForced},
		{name: "explicit never", Mode: ColorNever, explicit: true, Terminal: true, terminalName: "xterm", want: ColorDisabled},
		{name: "explicit auto beats NO_COLOR", Mode: ColorAuto, explicit: true, noColor: "1", Terminal: true, terminalName: "xterm", want: ColorDetected},
		{name: "explicit auto still respects pipe", Mode: ColorAuto, explicit: true, terminalName: "xterm", want: ColorDisabled},
		{name: "explicit auto still respects dumb", Mode: ColorAuto, explicit: true, Terminal: true, terminalName: "dumb", want: ColorDisabled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveColor(test.Mode, test.explicit, test.noColor, test.Terminal, test.terminalName)
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
		Mode        ColorMode
		explicit    bool
		Terminal    bool
		detected    colorprofile.Profile
		wantQueries int
		wantANSI    bool
		wantAccent  string
	}{
		{name: "detected terminal", Mode: ColorAuto, Terminal: true, detected: colorprofile.ANSI, wantQueries: 1, wantANSI: true},
		{
			name:        "detected truecolor terminal uses queried light background",
			Mode:        ColorAuto,
			Terminal:    true,
			detected:    colorprofile.TrueColor,
			wantQueries: 1,
			wantANSI:    true,
			wantAccent:  latteGreenSGR,
		},
		{name: "detected ASCII terminal", Mode: ColorAuto, Terminal: true, detected: colorprofile.ASCII, wantANSI: true},
		{name: "detected no-style terminal", Mode: ColorAuto, Terminal: true, detected: colorprofile.NoTTY},
		{
			name:       "forced pipe defaults to dark accents",
			Mode:       ColorAlways,
			explicit:   true,
			detected:   colorprofile.NoTTY,
			wantANSI:   true,
			wantAccent: frappeGreenSGR,
		},
		{name: "never terminal", Mode: ColorNever, explicit: true, Terminal: true, detected: colorprofile.TrueColor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mode := test.Mode
			queries := 0
			available := Presentation{
				Mode: &mode,
				Dependencies: PresentationDependencies{
					Environment:      func() []string { return []string{"TERM=xterm"} },
					IsTerminalWriter: func(io.Writer) bool { return test.Terminal },
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
			if err := human.WriteTaskMutation(VerbDone, task.Task{ID: 7, Title: "capture"}); err != nil {
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
