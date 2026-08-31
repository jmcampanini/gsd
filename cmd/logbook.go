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
		Long: `List done and cancelled tasks and projects, newest resolution first;
entries resolved at the same instant list projects before tasks and then
higher IDs first.

The human table has kind, id, title, status, and date columns, where
date is the resolution instant as a local calendar date; an empty result
prints nothing. With --json an array is written, each element having
kind (task or project), id, title, status, resolved_at (a UTC
timestamp), project_title, governing_area_id, governing_area_title, and
tags, with null for what does not apply; [] when empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
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
