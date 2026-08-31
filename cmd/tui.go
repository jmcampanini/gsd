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
		Long: `Open the navigator, a read-only browser of the database, in the
terminal. The root menu lists Inbox, Available, Logbook, Boards, and
Areas; opening one shows its rows, and opening a row shows its detail or
drills into an area, board, or project. Nothing the navigator does
changes the database; use the other commands to change anything.

Keys: j and k or the arrow keys move, enter or l opens the selection, h
or esc goes back a level, / starts a filter on the current list (typing
narrows it, esc or / ends editing, esc again clears it), and q or ctrl+c
quits from anywhere; esc at the root menu also quits. Operands are
rejected as a usage error (exit 2).

` + terminalContractHelp + `

The exit status is 0 when a key ends the session, and 1 with the error
on stderr when loading data fails.`,
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
