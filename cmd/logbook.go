package cmd

import (
	"time"

	"github.com/jmcampanini/gsd/internal/logbook"
	"github.com/spf13/cobra"
)

func newLogbookCommand(
	options *rootOptions,
	factory applicationFactory,
	location *time.Location,
) *cobra.Command {
	return &cobra.Command{
		Use:   "logbook",
		Short: "List completed and cancelled tasks and projects",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withLogbookApplication(command, options, factory, func(application logbook.Application) error {
				entries, err := application.List(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, entries, logbookWriter(location))
			})
		},
	}
}
