package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newStageRenameCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "rename BOARD OLD NEW",
		Short: "Rename a stage",
		Long: `Rename stage OLD on board BOARD to NEW. NEW must not be blank, and a
different stage on the board whose title matches NEW case-insensitively
is a conflict (exit 1); changing only the letter case of OLD is allowed.
Projects in the stage and task defers pointing at it follow the rename.

` + clihelp.NameGrammarHelp + `

On success one line reads '~ Renamed stage BOARD/OLD to BOARD/NEW'; with
--json the updated stage row is written (fields as in 'gsd board show
--help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(3),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageRenameResult, error) {
					return application.RenameStage(command.Context(), args[0], args[1], args[2])
				},
				func(renaming board.StageRenameResult) any { return renaming.Stage },
				render.HumanOutput.WriteRenamedStage,
			)
		},
	}
}
