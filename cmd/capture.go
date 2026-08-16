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
			if options.json {
				return usageError("--json is not supported by gsd capture; use gsd add TITLE for noninteractive capture")
			}
			if !options.presentation.isTerminalInput(command.InOrStdin()) {
				return usageError("gsd capture requires terminal input; use gsd add TITLE for noninteractive capture")
			}
			resolution := options.presentation.resolve(
				command.OutOrStdout(),
				command.Root().PersistentFlags().Changed("color"),
			)
			if !resolution.terminal {
				return usageError("gsd capture requires terminal output; use gsd add TITLE for noninteractive capture")
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				return runCapture(command.Context(), application, tui.ProgramOptions{
					Input:       command.InOrStdin(),
					Output:      command.OutOrStdout(),
					Environment: resolution.environment,
					Screen:      tui.ScreenAlt,
					Profile:     resolution.profile,
					Terminal:    resolution.terminal,
				})
			})
		},
	}

	return command
}
