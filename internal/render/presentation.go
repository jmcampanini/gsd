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

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// ColorDecision describes whether color is disabled, detected, or forced.
type ColorDecision uint8

const (
	ColorDisabled ColorDecision = iota
	ColorDetected
	ColorForced
)

// ResolveColor applies explicit color settings, NO_COLOR, and terminal detection.
func ResolveColor(
	mode ColorMode,
	explicit bool,
	noColor string,
	isTerminal bool,
	terminalName string,
) ColorDecision {
	if explicit {
		switch mode {
		case ColorAlways:
			return ColorForced
		case ColorNever:
			return ColorDisabled
		case ColorAuto:
			noColor = ""
		}
	}
	if noColor != "" || !isTerminal || terminalName == "dumb" {
		return ColorDisabled
	}
	return ColorDetected
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

// ColorResolution contains the selected profile and scrubbed terminal environment.
type ColorResolution struct {
	Profile     colorprofile.Profile
	Terminal    bool
	Environment []string
}

// IsTerminalInput reports whether the input stream is a terminal.
func (p Presentation) IsTerminalInput(reader io.Reader) bool {
	return p.Dependencies.IsTerminalReader(reader)
}

// Resolve selects a color profile and removes conflicting color environment variables.
func (p Presentation) Resolve(writer io.Writer, explicit bool) ColorResolution {
	environment := p.Dependencies.Environment()
	terminal := p.Dependencies.IsTerminalWriter(writer)
	scrubbed := scrubColorEnvironment(environment)
	profile := colorprofile.NoTTY
	switch ResolveColor(
		*p.Mode,
		explicit,
		environmentValue(environment, "NO_COLOR"),
		terminal,
		environmentValue(environment, "TERM"),
	) {
	case ColorForced:
		profile = colorprofile.TrueColor
	case ColorDetected:
		profile = p.Dependencies.DetectProfile(writer, scrubbed)
	}
	return ColorResolution{
		Profile:     profile,
		Terminal:    terminal,
		Environment: scrubbed,
	}
}

// Output creates a writer and queries the background only for a styled terminal.
func (p Presentation) Output(input io.Reader, writer io.Writer, explicit bool) HumanOutput {
	resolution := p.Resolve(writer, explicit)
	dark := true
	if tui.CanRenderColor(resolution.Profile, resolution.Terminal) {
		dark = p.Dependencies.HasDarkBackground(input, writer)
	}
	return NewHumanOutput(
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
