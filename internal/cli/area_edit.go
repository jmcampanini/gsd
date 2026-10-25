package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit an area",
		Long: `Change the title or note of an area. At least one of --title and --note
is required; without one the command fails with invalid_argument (exit
1) before the database is opened. An archived area can be edited.

` + clihelp.IdGrammarHelp + `

` + clihelp.NoteFlagHelp + `

On success one line reads '~ Edited: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note") {
				return apperr.New(
					apperr.InvalidArgument,
					"area edit requires --title or --note",
					nil,
				)
			}

			fields := area.EditFields{}
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, resolveErr := resolveNote(command, note)
				if resolveErr != nil {
					return resolveErr
				}
				fields.Note = &resolvedNote
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				edited, editErr := application.Edit(command.Context(), id, fields)
				if editErr != nil {
					return editErr
				}
				return writeCommandOutput(command, options, edited, render.AreaMutationWriter(render.VerbEdited))
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "area title")
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")

	return command
}
