package cli

import (
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := placementFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder an area",
		Long: `Move an area to a new position among all areas, the order 'gsd areas
list' shows. One placement flag is required (usage error, exit 2, when
none is given).

` + clihelp.IDGrammarHelp + `

` + clihelp.PlacementHelp + `

On success one line reads '~ Reordered: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, area.ParseID)
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, render.AreaMutationWriter(render.VerbReordered))
			})
		},
	}
	flags.register(command, "area")

	return command
}
