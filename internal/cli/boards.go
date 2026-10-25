package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newBoardsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("boards", "Manage boards", `Manage boards as a set: 'boards add NAME' creates one and 'boards list'
lists them. 'gsd board' holds the commands that act on one existing
board, and 'gsd stages' and 'gsd stage' manage the stages on a board.

A board is an ordered list of stages, addressed by name. A project can
sit in one stage of one board and moves through the stages with 'gsd
project move' or by promotion when a promoting task in it is completed.
Stage order also decides when a task's stage defer counts as reached.
Board titles are unique case-insensitively, and so are stage titles
within one board.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newBoardsAddCommand(options, factory),
		newBoardsListCommand(options, factory),
	)
	return command
}
