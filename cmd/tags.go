package cmd

import (
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "tags",
		Short: "Manage tags",
		Long: `Manage tags: 'tags add NAME' creates one, 'tags list' lists them with
usage counts, 'tags rename OLD NEW' renames one, and 'tags delete NAME'
removes one. Attach and detach tags with 'gsd tag', 'gsd untag', 'gsd
project tag', 'gsd project untag', 'gsd area tag', and 'gsd area untag'.

A tag is a label with a title that is unique case-insensitively. It can
be attached to any number of tasks, projects, and areas, and it must
exist before it is attached; attaching never creates one.

` + nameGrammarHelp + `

` + groupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError("tags requires a subcommand")
		},
	}
	command.AddCommand(
		newTagsAddCommand(options, factory),
		newTagsDeleteCommand(options, factory),
		newTagsListCommand(options, factory),
		newTagsRenameCommand(options, factory),
	)

	return command
}

func newTagsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "add NAME",
		Short: "Add a tag",
		Long: `Create a tag titled NAME and print it. NAME must not be blank
(invalid_argument, exit 1), and a tag whose title matches NAME
case-insensitively is a conflict (exit 1).

On success one line reads '+ Added tag NAME'; with --json the new tag
row is written: id, title, created_at, and updated_at, the timestamps in
UTC with millisecond precision.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				created, err := application.Add(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, created, humanOutput.writeAddedTag)
			})
		},
	}
}

func newTagsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tags",
		Long: `List every tag ordered by title, case-insensitively, then ID, each with
the number of tasks, projects, and areas it is attached to.

The human form is one line per tag, '#TITLE  COUNT'; an empty result
prints nothing. With --json an array of tag rows is written (id, title,
created_at, updated_at), each with "usage_count"; [] when empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				listed, err := application.List(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, listed, humanOutput.writeTagList)
			})
		},
	}
}

func newTagsRenameCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "rename OLD NEW",
		Short: "Rename a tag",
		Long: `Rename tag OLD to NEW everywhere it is attached. NEW must not be blank,
and a different tag whose title matches NEW case-insensitively is a
conflict (exit 1); changing only the letter case of OLD is allowed.

` + nameGrammarHelp + `

On success one line reads '~ Renamed tag OLD to NEW'; with --json the
updated tag row is written (id, title, created_at, updated_at).

` + outputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagOutput(command, options, factory,
				func(application tag.Application) (tag.Renaming, error) {
					return application.Rename(command.Context(), args[0], args[1])
				},
				func(renaming tag.Renaming) any { return renaming.Tag },
				func(output humanOutput, renaming tag.Renaming) error {
					return output.writeRenamedTag(renaming.PreviousTitle, renaming.Tag.Title)
				},
			)
		},
	}
}

func newTagsDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a tag",
		Long: `Delete tag NAME and detach it from every task, project, and area that
carries it, in the same transaction.

` + nameGrammarHelp + `

On success one line reads '− Deleted tag NAME (detached from N items)';
with --json {"tag":ROW,"detached":N} is written, where the tag row has
id, title, created_at, and updated_at.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				deletion, err := application.Delete(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, deletion, humanOutput.writeTagDeletion)
			})
		},
	}
}
