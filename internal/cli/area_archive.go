package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaArchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Archive an active area, recording archived_at. An archived area is
hidden from 'gsd areas list' by default, its tasks leave 'gsd
available', and it blocks changes to the projects and tasks it contains
until 'gsd area unarchive ID'.

` + clihelp.IDGrammarHelp + `

An area that is already archived is a conflict (exit 1).

On success one line reads '✗ Archived: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + clihelp.OutputContractHelp,
			short: "Archive an area",
			use:   "archive ID",
			verb:  render.VerbArchived,
		},
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Archive(ctx, id)
		},
	)
}
