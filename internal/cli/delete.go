package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Delete a task permanently, whatever its status and wherever it lives; a
resolved project or an archived area does not block deletion.

` + clihelp.IDGrammarHelp + `

On success one line reads '− Deleted: ID  TITLE'; with --json the deleted
task row is written (fields as in 'gsd show --help').

` + clihelp.OutputContractHelp,
			short: "Delete a task",
			use:   "delete ID",
			verb:  render.VerbDeleted,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Delete(ctx, id)
		},
	)
}
