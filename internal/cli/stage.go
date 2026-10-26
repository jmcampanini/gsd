package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/spf13/cobra"
)

func newStageCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("stage", "Manage a stage", `Act on one existing stage, named by its board and its own name. New
stages come from 'gsd stages', and 'gsd boards --help' describes boards
and stages.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newStageRenameCommand(options, factory),
		newStageReorderCommand(options, factory),
		newStageDeleteCommand(options, factory),
	)
	return command
}
