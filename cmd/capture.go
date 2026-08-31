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
		Long: `Open a one-line prompt in the terminal and add what is typed as an open
inbox task. Enter adds the text as the title (a blank title is ignored
and the prompt stays), and esc or ctrl+c cancels, also while the add is
in flight. The session ends after one add. This is the interactive form
of 'gsd add TITLE', which takes the same title noninteractively.

` + terminalContractHelp + `

The exit status is 0 after an add or a cancel. When the add fails, the
error shows in the prompt's footer, the next key press ends the session,
and the command exits 1 with the same error on stderr. Nothing but the
prompt is written to stdout.`,
		Args: cobra.NoArgs,
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
