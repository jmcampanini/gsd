package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a board",
		Long: `Delete a board and all of its stages. A board with any project on it,
open or resolved, is a conflict (exit 1) whose message counts them; move
those projects off with 'gsd project edit ID --no-board' or onto another
board first.

` + clihelp.NameGrammarHelp + `

On success one line reads '− Deleted: board NAME'; with --json
{"board":ROW,"stages":[ROW...]} is written with the deleted board and
its stages (fields as in 'gsd board show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				deletion, err := application.Delete(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, deletion, render.HumanOutput.WriteBoardDeletion)
			})
		},
	}
}
