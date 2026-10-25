package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from area ID.

` + clihelp.TaggingContractHelp,
			short: "Untag an area",
			use:   "untag ID NAME...",
			verb:  render.VerbUntagged,
		},
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}
