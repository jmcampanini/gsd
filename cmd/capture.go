package cmd

import (
	"context"

	"github.com/jmcampanini/gsd/internal/task"
	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/spf13/cobra"
)

type captureRunner func(context.Context, task.Application, tui.ProgramOptions) error

func newCaptureCommand(
	options *rootOptions,
	factory applicationFactory,
	runCapture captureRunner,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "capture",
		Short: "Capture an inbox task",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				resolution := options.presentation.resolve(
					command.OutOrStdout(),
					command.Root().PersistentFlags().Changed("color"),
				)
				return runCapture(command.Context(), application, tui.ProgramOptions{
					Input:       command.InOrStdin(),
					Output:      command.OutOrStdout(),
					Environment: resolution.environment,
					Screen:      tui.ScreenAlt,
					Color:       captureColorMode(resolution.decision),
				})
			})
		},
	}

	return command
}

func captureColorMode(decision colorDecision) tui.ColorMode {
	switch decision {
	case colorDisabled:
		return tui.ColorDisabled
	case colorDetected:
		return tui.ColorDetected
	case colorForced:
		return tui.ColorForced
	default:
		return tui.ColorDisabled
	}
}
