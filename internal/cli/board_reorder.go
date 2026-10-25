package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "reorder NAME",
		Short: "Reorder a board",
		Long: `Move a board to a new position among all boards, the order 'gsd boards
list' shows. One placement flag is required (usage error, exit 2, when
none is given).

` + clihelp.NameGrammarHelp + `

` + clihelp.NamedPlacementHelp + `

On success one line reads '~ Reordered: board NAME'; with --json the
updated board row is written (fields as in 'gsd board show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.placement(command)
			if err != nil {
				return err
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				reordered, err := application.Reorder(command.Context(), args[0], placement)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, reordered, render.BoardMutationWriter(render.VerbReordered))
			})
		},
	}
	flags.register(command, "board", true)
	return command
}
