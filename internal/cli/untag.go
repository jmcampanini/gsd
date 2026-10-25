package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from task ID.

` + clihelp.TaggingContractHelp,
			short: "Untag a task",
			use:   "untag ID NAME...",
			verb:  render.VerbUntagged,
		},
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}
