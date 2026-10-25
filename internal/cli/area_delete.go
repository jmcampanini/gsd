package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete an area",
		Long: `Delete an area permanently, archived or not. Without --recursive an area
that still contains projects or tasks is a conflict (exit 1) whose
message points to --recursive; with --recursive the tasks in its
projects, its projects, and its loose tasks are deleted first, in the
same transaction.

` + clihelp.IdGrammarHelp + `

On success one line reads '− Deleted: area ID  TITLE', followed by
'Deleted N projects:' and 'Deleted N tasks:' with one line per row when
--recursive removed any. With --json the deleted area row is written, or
with --recursive
{"area":ROW,"deleted_projects":[ROW...],"deleted_tasks":[ROW...]}, with
empty arrays when there was nothing to delete.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				deletion, deleteErr := application.Delete(command.Context(), id, recursive)
				if deleteErr != nil {
					if code, ok := apperr.CodeOf(deleteErr); ok && code == apperr.Conflict && !recursive {
						return apperr.New(
							apperr.Conflict,
							deleteErr.Error()+"; use --recursive to delete the area and its contents",
							deleteErr,
						)
					}
					return deleteErr
				}
				return renderResult(command, options, deletion,
					func(deletion area.Deletion) any {
						if recursive {
							return deletion
						}
						return deletion.Area
					},
					render.HumanOutput.WriteAreaDeletion,
				)
			})
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete contained projects and tasks")

	return command
}
