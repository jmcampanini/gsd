package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newInboxCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "List open inbox tasks",
		Long: `List open tasks that belong to no project and no area, ordered by
position and then ID.

The human table has id, title, and dates columns. dates shows 'due DATE'
(red, when color is on, once the date is today or earlier), 'defer DATE',
and 'defer→STAGE' as present, and a promoting task carries ↑ after its
title. An empty inbox prints nothing. With --json an array of task rows
is written (fields as in 'gsd show --help') plus project_title,
governing_area_id, and governing_area_title, which are null here; an
empty inbox is [].

` + clihelp.OutputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.Inbox(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, render.HumanOutput.WriteOpenTaskList)
			})
		},
	}
}
