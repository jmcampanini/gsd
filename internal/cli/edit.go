package cli

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	var dueOn string
	var noDue bool
	var deferUntil string
	var noDefer bool
	var deferStage string
	var noDeferStage bool
	var projectIDValue string
	var noProject bool
	var areaIDValue string
	var noArea bool
	var promotes bool
	var noPromotes bool
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a task",
		Long: `Change one or more fields of a task. At least one field flag is required;
without one the command fails (exit 1) before the database is opened.

` + clihelp.IDGrammarHelp + `

--title TEXT and --note replace the title and note. --due DATE and
--no-due set or clear the due date, and --defer DATE and --no-defer the
defer date. --defer-stage NAME and --no-defer-stage set or clear the
stage defer under the rules in 'gsd add --help'. --promotes and
--no-promotes turn project promotion on or off. --project ID,
--no-project, --area ID, and --no-area move the task between containers;
--no-project and --no-area send it to the inbox. Each pair is mutually
exclusive (usage error, exit 2), and --no-due, --no-defer,
--no-defer-stage, --no-project, --no-area, --promotes, and --no-promotes
cannot be given as false (usage error, exit 2).

` + clihelp.NoteFlagHelp + `

` + clihelp.DateGrammarHelp + `

` + clihelp.TaskContainerHelp + `

Moving a task out of its project, whether to another project, an area,
or the inbox, clears an existing stage defer unless --defer-stage is
given in the same command; the cleared task is listed under the result.
A task that does not move keeps its position, and blockers are checked
only when it moves.

` + clihelp.BlockerGuidanceHelp + `

On success one line reads '~ Edited: ID  TITLE', followed by a 'Cleared
stage defer' line when one was cleared. With --json the updated task row
is written, except that a command with --project, --no-project, --area,
or --no-area writes {"task":ROW,"cleared_defers":[ROW...]} instead, with
cleared_defers empty when nothing was cleared.

` + clihelp.OutputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := rejectFalseBooleanFlags(
				command,
				"no-due", "no-defer", "no-defer-stage", "no-project", "no-area", "promotes", "no-promotes",
			); err != nil {
				return err
			}
			id, err := task.ParseID(args[0])
			if err != nil {
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

			if !anyFlagChanged(
				command,
				"title", "note", "due", "no-due", "defer", "no-defer", "defer-stage", "no-defer-stage",
				"project", "no-project", "area", "no-area", "promotes", "no-promotes",
			) {
				return apperr.New(
					apperr.InvalidArgument,
					"edit requires --title, --note, --due, --no-due, --defer, --no-defer, --defer-stage, --no-defer-stage, --project, --no-project, --area, --no-area, --promotes, or --no-promotes",
					nil,
				)
			}

			fields := task.EditRequest{}
			fields.Project.Set = projectID
			fields.Project.Clear = noProject
			fields.Area.Set = areaID
			fields.Area.Clear = noArea
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, resolveErr := resolveNote(command, note)
				if resolveErr != nil {
					return resolveErr
				}
				fields.Note = &resolvedNote
			}
			if command.Flags().Changed("due") {
				fields.DueOn.Set = &dueOn
			}
			fields.DueOn.Clear = noDue
			if command.Flags().Changed("defer") {
				fields.DeferUntil.Set = &deferUntil
			}
			fields.DeferUntil.Clear = noDefer
			if command.Flags().Changed("defer-stage") {
				fields.DeferStage.Set = &deferStage
			}
			fields.DeferStage.Clear = noDeferStage
			if command.Flags().Changed("promotes") {
				fields.Promotes = &promotes
			}
			if noPromotes {
				value := false
				fields.Promotes = &value
			}
			containmentEdit := anyFlagChanged(command, "project", "no-project", "area", "no-area")

			return withTaskOutput(command, options, factory,
				func(application task.Application) (task.Edition, error) {
					return application.Edit(command.Context(), id, fields)
				},
				func(edited task.Edition) any {
					if containmentEdit {
						return edited
					}
					return edited.Task
				},
				render.HumanOutput.WriteTaskEdition,
			)
		},
	}
	command.Flags().StringVar(&title, "title", "", "task title")
	command.Flags().StringVar(&note, "note", "", "task note or - to read stdin")
	command.Flags().StringVar(&projectIDValue, "project", "", "project ID")
	command.Flags().BoolVar(&noProject, "no-project", false, "remove the task from its project")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().BoolVar(&noArea, "no-area", false, "remove the task from its area")
	command.Flags().StringVar(&dueOn, "due", "", "task due date")
	command.Flags().BoolVar(&noDue, "no-due", false, "clear the task due date")
	command.Flags().StringVar(&deferUntil, "defer", "", "task defer date")
	command.Flags().BoolVar(&noDefer, "no-defer", false, "clear the task defer date")
	command.Flags().StringVar(&deferStage, "defer-stage", "", "project stage until which to defer the task")
	command.Flags().BoolVar(&noDeferStage, "no-defer-stage", false, "clear the task stage defer")
	command.Flags().BoolVar(&promotes, "promotes", false, "advance the task's project when completed")
	command.Flags().BoolVar(&noPromotes, "no-promotes", false, "do not advance the task's project when completed")
	command.MarkFlagsMutuallyExclusive("due", "no-due")
	command.MarkFlagsMutuallyExclusive("defer", "no-defer")
	command.MarkFlagsMutuallyExclusive("defer-stage", "no-defer-stage")
	command.MarkFlagsMutuallyExclusive("promotes", "no-promotes")
	command.MarkFlagsMutuallyExclusive("project", "no-project")
	command.MarkFlagsMutuallyExclusive("area", "no-area")

	return command
}
