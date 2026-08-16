package cmd

import (
	"context"
	"time"

	"github.com/jmcampanini/gsd/internal/tui"
	"github.com/jmcampanini/gsd/internal/tui/navigator"
	"github.com/spf13/cobra"
)

type navigatorRunner func(
	context.Context,
	navigator.Dependencies,
	tui.ProgramOptions,
	*time.Location,
) error

func newTUICommand(
	options *rootOptions,
	factory applicationFactory,
	runNavigator navigatorRunner,
	location *time.Location,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive navigator",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageError("gsd tui takes no positional arguments; use the gsd CLI for noninteractive access")
			}
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			if options.json {
				return usageError("--json is not supported by gsd tui; use the gsd CLI for noninteractive access")
			}
			if !options.presentation.isTerminalInput(command.InOrStdin()) {
				return usageError("gsd tui requires terminal input; use the gsd CLI for noninteractive access")
			}
			resolution := options.presentation.resolve(
				command.OutOrStdout(),
				command.Root().PersistentFlags().Changed("color"),
			)
			if !resolution.terminal {
				return usageError("gsd tui requires terminal output; use the gsd CLI for noninteractive access")
			}

			return withApplications(command, options, factory, func(available applications) error {
				return runNavigator(command.Context(), navigator.Dependencies{
					Tasks:    available.tasks,
					Projects: available.projects,
					Areas:    available.areas,
					Boards:   available.boards,
					Logbook:  available.logbook,
				}, tui.ProgramOptions{
					Input:       command.InOrStdin(),
					Output:      command.OutOrStdout(),
					Environment: resolution.environment,
					Screen:      tui.ScreenAlt,
					Profile:     resolution.profile,
					Terminal:    resolution.terminal,
				}, location)
			})
		},
	}

	return command
}
