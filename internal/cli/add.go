package cli

import (
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var dueOn string
	var deferUntil string
	var deferStage string
	var projectIDValue string
	var areaIDValue string
	var noDeferStage bool
	var promotes bool
	var noPromotes bool
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a task",
		Long: `Create an open task titled TITLE and print it. TITLE must not be blank.

` + clihelp.TaskContainerHelp + `

` + clihelp.NoteFlagHelp + `

--due DATE sets the due date, and --defer DATE keeps the task out of
'gsd available' until that date.

` + clihelp.DateGrammarHelp + `

--defer-stage NAME keeps the task out of 'gsd available' until its
project reaches stage NAME on the project's board: it needs --project,
that project must be on a board, and NAME must be a stage of that board
(otherwise the command fails, exit 1). --promotes marks the task to
advance its project to the next stage of its board when the task is
completed. --no-defer-stage and --no-promotes name the defaults and are
mutually exclusive with their opposites; --promotes=false,
--no-promotes=false, and --no-defer-stage=false are usage errors
(exit 2).

` + clihelp.TagFlagHelp + `

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '+ Added task ID: TITLE' followed by any tags;
with --json the new task row is written (fields as in 'gsd show --help').

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := rejectFalseBooleanFlags(command, "no-defer-stage", "promotes", "no-promotes"); err != nil {
				return err
			}
			projectID, err := parseProjectIDFlag(command, projectIDValue)
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := task.AddRequest{
				ProjectID: projectID,
				AreaID:    areaID,
				Title:     args[0],
				Note:      resolvedNote,
				Promotes:  promotes,
				Tags:      tags,
			}
			if command.Flags().Changed("due") {
				fields.DueOn = &dueOn
			}
			if command.Flags().Changed("defer") {
				fields.DeferUntil = &deferUntil
			}
			if command.Flags().Changed("defer-stage") {
				fields.DeferStage = &deferStage
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				created, err := application.Add(command.Context(), fields)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, created, render.HumanOutput.WriteAddedTask)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "task note or - to read stdin")
	command.Flags().StringVar(&projectIDValue, "project", "", "project ID")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().StringVar(&dueOn, "due", "", "task due date")
	command.Flags().StringVar(&deferUntil, "defer", "", "task defer date")
	command.Flags().StringVar(&deferStage, "defer-stage", "", "project stage until which to defer the task")
	command.Flags().BoolVar(&noDeferStage, "no-defer-stage", false, "create the task without a stage defer")
	command.Flags().BoolVar(&promotes, "promotes", false, "advance the task's project when completed")
	command.Flags().BoolVar(&noPromotes, "no-promotes", false, "create the task without project promotion")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")
	command.MarkFlagsMutuallyExclusive("defer-stage", "no-defer-stage")
	command.MarkFlagsMutuallyExclusive("promotes", "no-promotes")

	return command
}
