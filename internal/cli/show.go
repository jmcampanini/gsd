package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a task",
		Long: `Print one task in full.

` + clihelp.IdGrammarHelp + `

The human form is a header line with a status glyph, the ID, and the
title (plus ↑ when the task promotes), then one row per field: project,
area, note, due on (red, when color is on, once the date is today or
earlier), defer until, defer stage, promotes, done at, cancelled at,
status, position, created at, updated at, and tags. An empty field shows
only its label, and a multi-line note is indented under its label. With
--json the task row is written: id, project_id, area_id, title, note,
defer_until, due_on, done_at, cancelled_at, status, position,
created_at, updated_at, defer_stage_id, promotes, and tags. Timestamps
are UTC with millisecond precision, dates are YYYY-MM-DD, absent values
are null, and tags is always an array.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				found, err := application.Show(command.Context(), id)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, found, render.HumanOutput.WriteTask)
			})
		},
	}
}
