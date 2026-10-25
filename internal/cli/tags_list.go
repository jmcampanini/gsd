package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tags",
		Long: `List every tag ordered by title, case-insensitively, then ID, each with
the number of tasks, projects, and areas it is attached to.

The human form is one line per tag, '#TITLE  COUNT'; an empty result
prints nothing. With --json an array of tag rows is written (id, title,
created_at, updated_at), each with "usage_count"; [] when empty.

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				listed, err := application.List(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, listed, render.HumanOutput.WriteTagList)
			})
		},
	}
}
