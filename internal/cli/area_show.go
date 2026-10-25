package cli

import (
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show an area",
		Long: `Print one area in full.

` + clihelp.IdGrammarHelp + `

The human form is a header line with a state glyph, the ID, and the
title, then one row per field: note, archived at, position, created at,
updated at, and tags. An empty field shows only its label, and a
multi-line note is indented under its label. With --json the area row is
written: id, title, note, archived_at, position, created_at, updated_at,
and tags. Timestamps are UTC with millisecond precision, archived_at is
null while the area is active, and tags is always an array.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				found, showErr := application.Show(command.Context(), id)
				if showErr != nil {
					return showErr
				}
				return writeCommandOutput(command, options, found, render.HumanOutput.WriteArea)
			})
		},
	}
}
