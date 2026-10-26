package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newAreasCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("areas", "Manage areas", `Manage areas as a set: create them and list them. 'gsd area' holds the
commands that act on one existing area.

An area groups projects and loose tasks. It has a title, a note, tags, a
position among all areas, and an archived state: an archived area is
hidden from 'gsd areas list' by default, its tasks leave 'gsd
available', and it blocks changes to the projects and tasks it contains
until it is unarchived.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newAreasAddCommand(options, factory),
		newAreasListCommand(options, factory),
	)

	return command
}
