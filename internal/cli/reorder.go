package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := placementFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder a task",
		Long: `Move a task to a new position among its siblings, the tasks in the same
container: the inbox, one project, or one area. One placement flag is
required (usage error, exit 2, when none is given).

` + clihelp.IDGrammarHelp + `

` + clihelp.PlacementHelp + `

On success one line reads '~ Reordered: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, task.ParseID)
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, render.TaskMutationWriter(render.VerbReordered))
			})
		},
	}
	flags.register(command, "task")

	return command
}
