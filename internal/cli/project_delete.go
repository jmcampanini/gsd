package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a project",
		Long: `Delete a project permanently, whatever its status or board. Without
--recursive a project that still contains tasks is a conflict (exit 1)
whose message points to --recursive; with --recursive its tasks are
deleted first, in the same transaction.

` + clihelp.IdGrammarHelp + `

On success one line reads '− Deleted: project ID  TITLE', followed by
'Deleted N tasks:' and one line per task when --recursive removed any.
With --json the deleted project row is written, or with --recursive
{"project":ROW,"deleted_tasks":[ROW...]}, with deleted_tasks empty when
there were none.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				deletion, err := application.Delete(command.Context(), id, recursive)
				if err != nil {
					if code, ok := apperr.CodeOf(err); ok && code == apperr.Conflict && !recursive {
						return apperr.New(
							apperr.Conflict,
							err.Error()+"; use --recursive to delete the project and its tasks",
							err,
						)
					}
					return err
				}
				return renderResult(command, options, deletion,
					func(deletion project.Deletion) any {
						if recursive {
							return deletion
						}
						return deletion.Project
					},
					render.HumanOutput.WriteProjectDeletion,
				)
			})
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete contained tasks")

	return command
}
