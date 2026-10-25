package cli

import (
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreasListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var archived bool
	var all bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List areas",
		Long: `List areas, ordered by position and then ID. Active areas are listed by
default; --archived lists only archived areas and --all lists both. The
two flags are mutually exclusive (usage error, exit 2).

The human table has id, title, and state columns, where state is
'archived' or blank; an empty result prints nothing. With --json an array
of area rows is written (fields as in 'gsd area show --help'), [] when
empty.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			slice := area.ListSliceActive
			if archived {
				slice = area.ListSliceArchived
			}
			if all {
				slice = area.ListSliceAll
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				areas, err := application.List(command.Context(), area.ListOptions{Slice: slice})
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, areas, render.HumanOutput.WriteAreaList)
			})
		},
	}
	command.Flags().BoolVar(&archived, "archived", false, "list archived areas")
	command.Flags().BoolVar(&all, "all", false, "list active and archived areas")
	command.MarkFlagsMutuallyExclusive("archived", "all")

	return command
}
