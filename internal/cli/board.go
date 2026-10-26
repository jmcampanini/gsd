package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newBoardCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("board", "Manage a board", `Act on one existing board by NAME. New boards and lists of them come from
'gsd boards', whose --help describes what a board is; 'gsd stages' and
'gsd stage' manage a board's stages.

`+clihelp.NameGrammarHelp+`

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newBoardShowCommand(options, factory),
		newBoardEditCommand(options, factory),
		newBoardReorderCommand(options, factory),
		newBoardDeleteCommand(options, factory),
	)
	return command
}
