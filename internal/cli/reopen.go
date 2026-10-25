package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Return a done or cancelled task to open, clearing done_at and
cancelled_at.

` + clihelp.IDGrammarHelp + `

A task that is already open is a conflict (exit 1).

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '~ Reopened: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + clihelp.OutputContractHelp,
			short: "Reopen a task",
			use:   "reopen ID",
			verb:  render.VerbReopened,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Reopen(ctx, id)
		},
	)
}
