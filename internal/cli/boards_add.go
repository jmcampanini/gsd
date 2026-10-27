package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var stages []string
	var note string
	command := &cobra.Command{
		Use:   "add NAME",
		Short: "Add a board",
		Long: `Create a board titled NAME with the stages given by --stage, in that
order, and print it. NAME and every stage name must not be blank, and at
least one --stage is required (an error otherwise, exit 1). A board whose
title matches NAME case-insensitively is a conflict (exit 1), as is a
--stage name repeated within the command; then nothing is created. The
board is placed last among all boards.

` + clihelp.NoteFlagHelp + `

On success one line reads '+ Board: NAME (STAGE → STAGE ...)'; with
--json the new board row is written (fields as in 'gsd board show
--help') without its stages, which 'gsd board show NAME' lists.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}
			fields := board.AddFields{Title: args[0], Note: resolvedNote, Stages: stages}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.Addition, error) {
					return application.Add(command.Context(), fields)
				},
				func(addition board.Addition) any { return addition.Board },
				render.HumanOutput.WriteAddedBoard,
			)
		},
	}
	command.Flags().StringArrayVar(&stages, "stage", nil, "stage name (repeatable)")
	command.Flags().StringVar(&note, "note", "", "board note or - to read stdin")
	return command
}
