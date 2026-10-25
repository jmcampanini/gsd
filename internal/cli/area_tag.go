package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to area ID.

` + clihelp.TaggingContractHelp,
			short: "Tag an area",
			use:   "tag ID NAME...",
			verb:  render.VerbTagged,
		},
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}
