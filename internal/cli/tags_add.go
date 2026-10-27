package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/spf13/cobra"
)

func newTagsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "add NAME",
		Short: "Add a tag",
		Long: `Create a tag titled NAME and print it. NAME must not be blank
(exit 1), and a tag whose title matches NAME case-insensitively is a
conflict (exit 1).

On success one line reads '+ Added tag NAME'; with --json the new tag
row is written: id, title, created_at, and updated_at, the timestamps in
UTC with millisecond precision.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withTagApplication(command, options, factory, func(application tag.Application) error {
				created, err := application.Add(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, created, render.HumanOutput.WriteAddedTag)
			})
		},
	}
}
