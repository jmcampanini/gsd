package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to task ID.

` + clihelp.TaggingContractHelp,
			short: "Tag a task",
			use:   "tag ID NAME...",
			verb:  render.VerbTagged,
		},
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}
