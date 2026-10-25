package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show a board",
		Long: `Print a board with its stages and the open projects in each stage.

` + clihelp.NameGrammarHelp + `

The human form is a headline 'NAME  STAGE → STAGE ...' (or '(no
stages)'), then one line per open project under its stage, '◆ ID  TITLE
DONE/TOTAL', where DONE counts the project's done tasks and TOTAL its
tasks that are not cancelled, or '(empty)' for a stage without open
projects. Resolved projects are not shown. With --json
{"board":ROW,"stages":[...]} is written, where the board row has id,
title, note, position, created_at, and updated_at, and each stage has
id, board_id, title, position, created_at, updated_at, and "projects",
an array of project rows (fields as in 'gsd project show --help') each
carrying "progress":{"done":N,"total":N}.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				shown, err := application.Show(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, shown, render.HumanOutput.WriteBoard)
			})
		},
	}
}
