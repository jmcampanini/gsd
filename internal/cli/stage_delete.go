package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newStageDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete BOARD NAME",
		Short: "Delete a stage",
		Long: `Delete stage NAME from board BOARD. A stage with any project in it, open
or resolved, is a conflict (exit 1) whose message counts them. Task stage
defers pointing at the stage are cleared in the same transaction and the
cleared tasks are listed.

` + clihelp.NameGrammarHelp + `

On success one line reads '− Deleted: stage BOARD/NAME', followed by a
'Cleared stage defer' line per task. With --json
{"stage":ROW,"cleared_defers":[ROW...]} is written, with cleared_defers
empty when no task deferred to the stage.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				result, err := application.DeleteStage(command.Context(), args[0], args[1])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, result, render.HumanOutput.WriteStageDeletion)
			})
		},
	}
}
