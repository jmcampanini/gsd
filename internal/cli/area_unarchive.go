package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaUnarchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Return an archived area to active, clearing archived_at.

` + clihelp.IdGrammarHelp + `

An area that is already active is a conflict (exit 1).

On success one line reads '~ Unarchived: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + clihelp.OutputContractHelp,
			short: "Unarchive an area",
			use:   "unarchive ID",
			verb:  render.VerbUnarchived,
		},
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Unarchive(ctx, id)
		},
	)
}
