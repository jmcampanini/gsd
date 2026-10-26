package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("tags", "Manage tags", `Manage tags as a set, apart from the rows that carry them. Attaching and
detaching happen on the entity: 'gsd tag' and 'gsd untag' for tasks, and
the tag and untag subcommands of 'gsd project' and 'gsd area'.

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
