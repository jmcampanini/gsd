package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectResolveCommand(
		options,
		factory,
		commandSpec{
			long: `Mark an open project cancelled, recording cancelled_at, and cancel every
open task in it in the same transaction.

` + clihelp.IDGrammarHelp + `

A project that is already done or cancelled is a conflict (exit 1), as
is one filed under an archived area.

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '✗ Cancelled: project ID  TITLE', followed by
'Cancelled N open tasks:' and one line per task when any were open. With
--json {"project":ROW,"cancelled_tasks":[ROW...]} is written, with
cancelled_tasks empty when none were open.

` + clihelp.OutputContractHelp,
			short: "Cancel a project",
			use:   "cancel ID",
			verb:  render.VerbCancelled,
		},
		project.ExitCancelled,
	)
}
