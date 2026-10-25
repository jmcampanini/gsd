package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Mark an open task cancelled, recording cancelled_at.

` + clihelp.IdGrammarHelp + `

A task that is already done or cancelled is a conflict (exit 1).

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '✗ Cancelled: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + clihelp.OutputContractHelp,
			short: "Cancel a task",
			use:   "cancel ID",
			verb:  render.VerbCancelled,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Cancel(ctx, id)
		},
	)
}
