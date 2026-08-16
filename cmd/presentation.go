package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/term"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/cobra"
)

type colorMode string

const (
	colorAuto   colorMode = "auto"
	colorAlways colorMode = "always"
	colorNever  colorMode = "never"
)

type colorValue struct {
	mode *colorMode
}

func (v colorValue) Set(value string) error {
	mode := colorMode(value)
	switch mode {
	case colorAuto, colorAlways, colorNever:
		*v.mode = mode
		return nil
	default:
		return fmt.Errorf("invalid color mode %q: must be auto, always, or never", value)
	}
}

func (v colorValue) String() string {
	if v.mode == nil || *v.mode == "" {
		return string(colorAuto)
	}
	return string(*v.mode)
}

func (colorValue) Type() string {
	return "color"
}

type colorDecision uint8

const (
	colorDisabled colorDecision = iota
	colorDetected
	colorForced
)

func resolveColor(
	mode colorMode,
	explicit bool,
	noColor string,
	isTerminal bool,
	terminalName string,
) colorDecision {
	if explicit {
		switch mode {
		case colorAlways:
			return colorForced
		case colorNever:
			return colorDisabled
		case colorAuto:
			if !isTerminal || terminalName == "dumb" {
				return colorDisabled
			}
			return colorDetected
		}
	}
	if noColor != "" || !isTerminal || terminalName == "dumb" {
		return colorDisabled
	}
	return colorDetected
}

type presentationDependencies struct {
	environment       func() []string
	isTerminalReader  func(io.Reader) bool
	isTerminalWriter  func(io.Writer) bool
	detectProfile     func(io.Writer, []string) colorprofile.Profile
	hasDarkBackground func(io.Reader, io.Writer) bool
	now               func() time.Time
}

func fileIsTerminal(stream any) bool {
	file, ok := stream.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}

func defaultPresentationDependencies() presentationDependencies {
	return presentationDependencies{
		environment:      os.Environ,
		isTerminalReader: func(reader io.Reader) bool { return fileIsTerminal(reader) },
		isTerminalWriter: func(writer io.Writer) bool { return fileIsTerminal(writer) },
		detectProfile:    colorprofile.Detect,
		hasDarkBackground: func(input io.Reader, output io.Writer) bool {
			in, inputOK := input.(term.File)
			out, outputOK := output.(term.File)
			if !inputOK || !outputOK {
				return true
			}
			return lipgloss.HasDarkBackground(in, out)
		},
		now: time.Now,
	}
}

type presentation struct {
	mode         *colorMode
	dependencies presentationDependencies
	location     *time.Location
}

type colorResolution struct {
	profile     colorprofile.Profile
	terminal    bool
	environment []string
}

func (p presentation) isTerminalInput(reader io.Reader) bool {
	return p.dependencies.isTerminalReader(reader)
}

func (p presentation) resolve(writer io.Writer, explicit bool) colorResolution {
	environment := p.dependencies.environment()
	terminal := p.dependencies.isTerminalWriter(writer)
	scrubbed := scrubColorEnvironment(environment)
	decision := resolveColor(
		*p.mode,
		explicit,
		environmentValue(environment, "NO_COLOR"),
		terminal,
		environmentValue(environment, "TERM"),
	)
	profile := colorprofile.NoTTY
	switch decision {
	case colorForced:
		profile = colorprofile.TrueColor
	case colorDetected:
		profile = p.dependencies.detectProfile(writer, scrubbed)
	case colorDisabled:
	}
	return colorResolution{
		profile:     profile,
		terminal:    terminal,
		environment: scrubbed,
	}
}

func (p presentation) output(command *cobra.Command) humanOutput {
	writer := command.OutOrStdout()
	resolution := p.resolve(writer, command.Root().PersistentFlags().Changed("color"))
	dark := true
	if tui.CanRenderColor(resolution.profile, resolution.terminal) {
		dark = p.dependencies.hasDarkBackground(command.InOrStdin(), writer)
	}
	return newHumanOutput(
		&colorprofile.Writer{Forward: writer, Profile: resolution.profile},
		dark,
		p.dependencies.now().In(p.location).Format(time.DateOnly),
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
