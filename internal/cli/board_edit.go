package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newBoardEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit NAME",
		Short: "Edit a board",
		Long: `Change the title or note of a board. At least one of --title and --note
is required; without one the command fails (exit 1) before the database
is opened. A new title already used by another board, compared
case-insensitively, is a conflict (exit 1).

` + clihelp.NameGrammarHelp + `

` + clihelp.NoteFlagHelp + `

On success one line reads '~ Edited: board TITLE'; with --json the
updated board row is written (fields as in 'gsd board show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !anyFlagChanged(command, "title", "note") {
				return apperr.New(
					apperr.InvalidArgument,
					"board edit requires --title or --note",
					nil,
				)
			}

			fields := board.EditFields{}
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, err := resolveNote(command, note)
				if err != nil {
					return err
				}
				fields.Note = &resolvedNote
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				edited, err := application.Edit(command.Context(), args[0], fields)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, edited, render.BoardMutationWriter(render.VerbEdited))
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "board title")
	command.Flags().StringVar(&note, "note", "", "board note or - to read stdin")
	return command
}
