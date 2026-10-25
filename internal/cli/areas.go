package cli

import (
	"github.com/jmcampanini/gsd/internal/clierror"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newAreasCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "areas",
		Short: "Manage areas",
		Long: `Manage areas as a set: 'areas add TITLE' creates one and 'areas list'
lists them. 'gsd area' holds the commands that act on one existing area.

An area groups projects and loose tasks. It has a title, a note, tags, a
position among all areas, and an archived state: an archived area is
hidden from 'gsd areas list' by default, its tasks leave 'gsd
available', and it blocks changes to the projects and tasks it contains
until it is unarchived.

` + clihelp.GroupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return clierror.Usage("areas requires a subcommand")
		},
	}
	command.AddCommand(
		newAreasAddCommand(options, factory),
		newAreasListCommand(options, factory),
	)

	return command
}
