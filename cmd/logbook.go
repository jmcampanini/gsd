package cmd

import (
	"fmt"
	"strconv"
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
				if options.json {
					return writeJSON(command.OutOrStdout(), entries)
				}

				return options.presentation.output(command).writeLogbook(entries, location)
			})
		},
	}
}

func (o humanOutput) writeLogbook(entries []logbook.Entry, location *time.Location) error {
	if len(entries) == 0 {
		return nil
	}

	rows := make([][]string, 0, len(entries))
	for _, entry := range entries {
		resolvedAt, err := time.Parse(time.RFC3339Nano, entry.ResolvedAt)
		if err != nil {
			return fmt.Errorf("parse logbook resolved_at for %s %d: %w", entry.Kind, entry.ID, err)
		}
		rows = append(rows, []string{
			humanText(entry.Kind, false),
			strconv.FormatInt(entry.ID, 10),
			humanText(entry.Title, false),
			o.statusWord(entry.Status),
			resolvedAt.In(location).Format(time.DateOnly),
		})
	}

	return o.writeCollection(
		[]string{"kind", "id", "title", "status", "date"},
		rows,
		1,
		0, 1, 4,
	)
}
