package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newDoneCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "done ID",
		Short: "Complete a task",
		Long: `Mark an open task done, recording done_at.

` + clihelp.IdGrammarHelp + `

A task that is already done or cancelled is a conflict (exit 1). When the
task promotes and its project is on a board, the project moves to the
next stage of that board in the same transaction and is placed last
there; a project already at the last stage stays where it is and the
result says so.

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '✓ Done: ID  TITLE', followed by '~ Promoted:
◆ PID  PTITLE → STAGE' when the project moved, or that line ending in
'(already at last stage)'. With --json the task row is written, except
that a promoting task writes {"task":ROW,"promoted_project":PROJECT},
where promoted_project is null when no stage changed.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}
			return withTaskOutput(command, options, factory,
				func(application task.Application) (task.Completion, error) {
					return application.Done(command.Context(), id)
				},
				func(completion task.Completion) any {
					if completion.Task.Promotes {
						return completion
					}
					return completion.Task
				},
				render.HumanOutput.WriteTaskCompletion,
			)
		},
	}
}
