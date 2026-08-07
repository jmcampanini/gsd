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
	isTerminal        func(io.Writer) bool
	detectProfile     func(io.Writer, []string) colorprofile.Profile
	hasDarkBackground func(io.Reader, io.Writer) bool
	now               func() time.Time
}

func defaultPresentationDependencies() presentationDependencies {
	return presentationDependencies{
		environment: os.Environ,
		isTerminal: func(writer io.Writer) bool {
			file, ok := writer.(interface{ Fd() uintptr })
			return ok && term.IsTerminal(file.Fd())
		},
		detectProfile: colorprofile.Detect,
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
	decision    colorDecision
	terminal    bool
	environment []string
}

func (p presentation) resolve(writer io.Writer, explicit bool) colorResolution {
	environment := p.dependencies.environment()
	terminal := p.dependencies.isTerminal(writer)
	return colorResolution{
		decision: resolveColor(
			*p.mode,
			explicit,
			environmentValue(environment, "NO_COLOR"),
			terminal,
			environmentValue(environment, "TERM"),
		),
		terminal:    terminal,
		environment: scrubColorEnvironment(environment),
	}
}

func (p presentation) profile(writer io.Writer, explicit bool) (colorprofile.Profile, bool) {
	resolution := p.resolve(writer, explicit)
	switch resolution.decision {
	case colorDisabled:
		return colorprofile.NoTTY, resolution.terminal
	case colorForced:
		return colorprofile.TrueColor, resolution.terminal
	case colorDetected:
		return p.dependencies.detectProfile(writer, resolution.environment), resolution.terminal
	default:
		return colorprofile.NoTTY, resolution.terminal
	}
}

func (p presentation) output(command *cobra.Command) humanOutput {
	writer := command.OutOrStdout()
	profile, terminal := p.profile(writer, command.Root().PersistentFlags().Changed("color"))
	dark := true
	if profile >= colorprofile.ANSI && terminal {
		dark = p.dependencies.hasDarkBackground(command.InOrStdin(), writer)
	}
	return newHumanOutput(
		&colorprofile.Writer{Forward: writer, Profile: profile},
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
