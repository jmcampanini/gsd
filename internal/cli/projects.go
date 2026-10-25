package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newProjectsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("projects", "Manage projects", `Manage projects as a set: 'projects add TITLE' creates one and 'projects
list' lists them. 'gsd project' holds the commands that act on one
existing project.

A project groups tasks. It has a title, a note, tags, a status of open,
done, or cancelled, and a position among the projects filed under the
same area (or among those with no area). It may sit on one board, in one
stage of that board, where it has a separate position among the projects
in that stage. Resolving a project cancels its open tasks, and a resolved
project blocks changes to its tasks until it is reopened.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newProjectsAddCommand(options, factory),
		newProjectsListCommand(options, factory),
	)

	return command
}
