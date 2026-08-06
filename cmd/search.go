package cmd

import (
	"github.com/jmcampanini/gsd/internal/search"
	"github.com/spf13/cobra"
)

func newSearchCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "search EXPR",
		Short: "Search tasks, projects, and areas",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withSearchApplication(command, options, factory, func(application search.Application) error {
				hits, err := application.Search(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, hits, humanOutput.writeSearchHits)
			})
		},
	}
}
