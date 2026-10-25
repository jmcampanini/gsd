package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a tag",
		Long: `Delete tag NAME and detach it from every task, project, and area that
carries it, in the same transaction.

` + clihelp.NameGrammarHelp + `

On success one line reads '− Deleted tag NAME (detached from N items)';
with --json {"tag":ROW,"detached":N} is written, where the tag row has
id, title, created_at, and updated_at.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				deletion, err := application.Delete(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, deletion, render.HumanOutput.WriteTagDeletion)
			})
		},
	}
}
