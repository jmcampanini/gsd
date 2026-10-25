package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("tags", "Manage tags", `Manage tags: 'tags add NAME' creates one, 'tags list' lists them with
usage counts, 'tags rename OLD NEW' renames one, and 'tags delete NAME'
removes one. Attach and detach tags with 'gsd tag', 'gsd untag', 'gsd
project tag', 'gsd project untag', 'gsd area tag', and 'gsd area untag'.

A tag is a label with a title that is unique case-insensitively. It can
be attached to any number of tasks, projects, and areas, and it must
exist before it is attached; attaching never creates one.

`+clihelp.NameGrammarHelp+`

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newTagsAddCommand(options, factory),
		newTagsDeleteCommand(options, factory),
		newTagsListCommand(options, factory),
		newTagsRenameCommand(options, factory),
	)

	return command
}

func withTagApplication(
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(tag.Application) error,
) error {
	return withApplications(command, options, factory, func(available applications) error {
		return run(available.tags)
	})
}
