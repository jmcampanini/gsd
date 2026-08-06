package cmd

import (
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/spf13/cobra"
)

func newSearchCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var related bool
	command := &cobra.Command{
		Use:   "search EXPR",
		Short: "Search tasks, projects, and areas",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withSearchApplication(command, options, factory, func(application search.Application) error {
				hits, err := application.Search(command.Context(), args[0], related)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, hits, humanOutput.writeSearchHits)
			})
		},
	}
	command.Flags().BoolVar(&related, "related", false, "include matches from project and area context")
	return command
}
