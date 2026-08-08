package cmd

import (
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "tags",
		Short: "Manage tags",
		Args:  cobra.NoArgs,
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.NoArgs,
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
		Args:  cobra.ExactArgs(2),
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
		Args:  cobra.ExactArgs(1),
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
