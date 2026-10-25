package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a project",
		Long: `Print one project in full.

` + clihelp.IdGrammarHelp + `

The human form is a header line with a status glyph, the ID, and the
title, then one row per field: area, board (as BOARD/STAGE), note, done
at, cancelled at, status, position, created at, updated at, and tags. An
empty field shows only its label, and a multi-line note is indented
under its label. With --json the project row is written: id, area_id,
title, note, done_at, cancelled_at, status, position, created_at,
updated_at, stage_id, stage_position, and tags; the board and stage
appear only as stage_id and stage_position. Timestamps are UTC with
millisecond precision, absent values are null, and tags is always an
array.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Detail, error) {
					return application.Show(command.Context(), id)
				},
				func(found project.Detail) any { return found.Project },
				render.HumanOutput.WriteProject,
			)
		},
	}
}
