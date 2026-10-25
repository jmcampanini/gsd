package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newStageReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "reorder BOARD NAME",
		Short: "Reorder a stage",
		Long: `Move stage NAME to a new position among the stages of board BOARD. One
placement flag is required (usage error, exit 2, when none is given).
Stage order decides the next stage for promotion and whether a task's
stage defer counts as reached, so reordering can change 'gsd available'.

` + clihelp.NameGrammarHelp + `

` + clihelp.NamedPlacementHelp + `

On success one line reads '~ Reordered: stage BOARD/NAME'; with --json
the updated stage row is written (fields as in 'gsd board show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.placement(command)
			if err != nil {
				return err
			}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageResult, error) {
					return application.ReorderStage(command.Context(), args[0], args[1], placement)
				},
				func(result board.StageResult) any { return result.Stage },
				render.StageMutationWriter(render.VerbReordered),
			)
		},
	}
	flags.register(command, "stage", true)
	return command
}
