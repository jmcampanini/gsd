package render

import (
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/jmcampanini/gsd/internal/tui"
)

// ColorMode selects automatic, forced, or disabled color.
type ColorMode string

// The --color values. ColorAuto follows NO_COLOR and terminal detection.
const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// colorDecision describes whether color is disabled, detected, or forced.
type colorDecision uint8

const (
	colorDisabled colorDecision = iota
	colorDetected
	colorForced
)

// resolveColor applies the --color mode, then for auto NO_COLOR and terminal
// detection. Only always and never override those inputs; an explicit auto
// is the same as the default.
func resolveColor(
	mode ColorMode,
	noColor string,
	isTerminal bool,
	terminalName string,
) colorDecision {
	switch mode {
	case ColorAlways:
		return colorForced
	case ColorNever:
		return colorDisabled
	}
	if noColor != "" || !isTerminal || terminalName == "dumb" {
		return colorDisabled
	}
	return colorDetected
}

// hasCursorControl reports whether TERM names a terminal that accepts cursor
// movement and the alternate screen. An empty or dumb TERM reports neither.
// It is a capability of its own: a terminal stream may still lack it, and a
// color profile never implies it.
func hasCursorControl(terminalName string) bool {
	return terminalName != "" && terminalName != "dumb"
}

// PresentationDependencies supplies terminal, environment, and clock observations.
type PresentationDependencies struct {
	Environment       func() []string
	IsTerminalReader  func(io.Reader) bool
	IsTerminalWriter  func(io.Writer) bool
	DetectProfile     func(io.Writer, []string) colorprofile.Profile
	HasDarkBackground func(io.Reader, io.Writer) bool
	Now               func() time.Time
}

func fileIsTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}

// DefaultPresentationDependencies uses the process environment, terminal streams, and local clock.
func DefaultPresentationDependencies() PresentationDependencies {
	return PresentationDependencies{
		Environment:      os.Environ,
		IsTerminalReader: func(reader io.Reader) bool { return fileIsTerminal(reader) },
		IsTerminalWriter: func(writer io.Writer) bool { return fileIsTerminal(writer) },
		DetectProfile:    colorprofile.Detect,
		HasDarkBackground: func(input io.Reader, output io.Writer) bool {
			in, inputOK := input.(term.File)
			out, outputOK := output.(term.File)
			if !inputOK || !outputOK {
				return true
			}
			return lipgloss.HasDarkBackground(in, out)
		},
		Now: time.Now,
	}
}

// Presentation resolves terminal capabilities and creates human output writers.
type Presentation struct {
	Mode         *ColorMode
	Dependencies PresentationDependencies
	Location     *time.Location
}

// ColorResolution contains the selected profile, the capabilities of the
// destination stream, and the scrubbed terminal environment. Terminal,
// Profile, and CursorControl are observed separately and never derived from
// one another.
type ColorResolution struct {
	Profile       colorprofile.Profile
	Terminal      bool
	CursorControl bool
	Environment   []string
}

// IsTerminalInput reports whether the input stream is a terminal.
func (p Presentation) IsTerminalInput(reader io.Reader) bool {
	return p.Dependencies.IsTerminalReader(reader)
}

// Resolve selects a color profile for writer, observes the stream's terminal
// capabilities, and removes conflicting color environment variables.
func (p Presentation) Resolve(writer io.Writer) ColorResolution {
	environment := p.Dependencies.Environment()
	terminal := p.Dependencies.IsTerminalWriter(writer)
	terminalName := environmentValue(environment, "TERM")
	scrubbed := scrubColorEnvironment(environment)
	profile := colorprofile.NoTTY
	switch resolveColor(
		*p.Mode,
		environmentValue(environment, "NO_COLOR"),
		terminal,
		terminalName,
	) {
	case colorForced:
		profile = colorprofile.TrueColor
	case colorDetected:
		profile = p.Dependencies.DetectProfile(writer, scrubbed)
	}
	return ColorResolution{
		Profile:       profile,
		Terminal:      terminal,
		CursorControl: hasCursorControl(terminalName),
		Environment:   scrubbed,
	}
}

// Output creates a writer and queries the background only for a styled terminal.
func (p Presentation) Output(input io.Reader, writer io.Writer) HumanOutput {
	resolution := p.Resolve(writer)
	dark := true
	if tui.CanRenderColor(resolution.Profile, resolution.Terminal) {
		dark = p.Dependencies.HasDarkBackground(input, writer)
	}
	return newHumanOutput(
		&colorprofile.Writer{Forward: writer, Profile: resolution.Profile},
		dark,
		p.Dependencies.Now().In(p.Location).Format(time.DateOnly),
	)
}

func environmentValue(environment []string, name string) string {
	value := ""
	for _, entry := range environment {
		key, current, found := strings.Cut(entry, "=")
		if found && key == name {
			value = current
		}
	}
	return value
}

func scrubColorEnvironment(environment []string) []string {
	scrubbed := make([]string, 0, len(environment))
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "NO_COLOR", "FORCE_COLOR", "CLICOLOR", "CLICOLOR_FORCE":
			continue
		default:
			scrubbed = append(scrubbed, entry)
		}
	}
	return scrubbed
}
