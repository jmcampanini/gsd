package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := placementFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder a project",
		Long: `Move a project to a new position among its siblings, the projects filed
under the same area or, for a project without one, those with no area.
This is the order 'gsd projects list' shows; the position within a board
stage is separate and changed by 'gsd project move'. One placement flag
is required (usage error, exit 2, when none is given).

` + clihelp.IDGrammarHelp + `

` + clihelp.PlacementHelp + `

On success one line reads '~ Reordered: project ID  TITLE'; with --json
the updated project row is written (fields as in 'gsd project show
--help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, project.ParseID)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, render.ProjectMutationWriter(render.VerbReordered))
			})
		},
	}
	flags.register(command, "project")

	return command
}
