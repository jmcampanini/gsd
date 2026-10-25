package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newStagesAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "add BOARD NAME",
		Short: "Add a stage",
		Long: `Add stage NAME to board BOARD and print it. The stage goes last unless a
placement flag says otherwise. A stage on that board whose title matches
NAME case-insensitively is a conflict (exit 1).

` + clihelp.NameGrammarHelp + `

Placement is optional here, and the siblings are BOARD's stages.

` + clihelp.NamedPlacementHelp + `

On success one line reads '+ Added stage BOARD/NAME'; with --json the new
stage row is written (fields as in 'gsd board show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.optionalPlacement(command)
			if err != nil {
				return err
			}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageResult, error) {
					return application.AddStage(command.Context(), args[0], args[1], placement)
				},
				func(result board.StageResult) any { return result.Stage },
				render.HumanOutput.WriteAddedStage,
			)
		},
	}
	flags.register(command, "stage", false)
	return command
}
