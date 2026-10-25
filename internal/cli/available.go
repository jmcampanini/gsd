package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newAvailableCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "available",
		Short: "List available tasks",
		Long: `List open tasks that can be worked on now, ordered by position and then
ID. A task qualifies when its project, if any, is open; its governing
area (its own area, or its project's) is not archived; its defer date is
absent or not after today; and its stage defer is absent or names a
stage its project has already reached on the same board. Inbox tasks
that pass the date rule are included.

Output has the shape of 'gsd inbox', with project_title,
governing_area_id, and governing_area_title filled in when the task has
a project or a governing area.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.Available(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, render.HumanOutput.WriteOpenTaskList)
			})
		},
	}
}
