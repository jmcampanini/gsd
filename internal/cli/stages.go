package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newStagesCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("stages", "Manage stages", `Add stages to a board with 'stages add BOARD NAME'. 'gsd stage' holds the
commands that act on one existing stage, and 'gsd boards --help'
describes boards and stages.

`+clihelp.GroupContractHelp)
	command.AddCommand(newStagesAddCommand(options, factory))
	return command
}
