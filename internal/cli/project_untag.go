package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from project ID.

` + clihelp.TaggingContractHelp,
			short: "Untag a project",
			use:   "untag ID NAME...",
			verb:  render.VerbUntagged,
		},
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}
