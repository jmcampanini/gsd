package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newBoardCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("board", "Manage a board", `Act on one existing board by NAME: show, edit, reorder, and delete. New
boards come from 'gsd boards add' and lists from 'gsd boards list';
'gsd boards --help' describes what a board is, and 'gsd stages' and 'gsd
stage' manage a board's stages.

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
