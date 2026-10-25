package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to project ID.

` + clihelp.TaggingContractHelp,
			short: "Tag a project",
			use:   "tag ID NAME...",
			verb:  render.VerbTagged,
		},
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}
