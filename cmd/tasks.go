package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
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

` + taskContainerHelp + `

` + noteFlagHelp + `

--due DATE sets the due date, and --defer DATE keeps the task out of
'gsd available' until that date.

` + dateGrammarHelp + `

--defer-stage NAME keeps the task out of 'gsd available' until its
project reaches stage NAME on the project's board: it needs --project,
that project must be on a board, and NAME must be a stage of that board
(invalid_argument otherwise, or not_found when no board has such a
stage). --promotes marks the task to advance its project to the next
stage of its board when the task is completed. --no-defer-stage and
--no-promotes name the defaults and are mutually exclusive with their
opposites; --promotes=false, --no-promotes=false, and
--no-defer-stage=false are usage errors (exit 2).

` + tagFlagHelp + `

` + blockerGuidanceHelp + `

On success one line reads '+ Added task ID: TITLE' followed by any tags;
with --json the new task row is written (fields as in 'gsd show --help').

` + outputContractHelp,
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
				return writeCommandOutput(command, options, created, humanOutput.writeAddedTask)
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

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.Inbox(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, humanOutput.writeOpenTaskList)
			})
		},
	}
}

func newAvailableCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "available",
		Short: "List available tasks",
		Long: `List open tasks that can be worked on now, ordered by position and then
ID. A task qualifies when its project, if any, is open; its governing
area (its own area, or its project's) is not archived; its defer date is
absent or not after today; and its stage defer is absent or names a
stage its project has already reached on the same board. Inbox tasks
that pass the date rule are included.

Output has the shape of 'gsd inbox', with project_title,
governing_area_id, and governing_area_title filled in when the task has
a project or a governing area.

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.Available(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, humanOutput.writeOpenTaskList)
			})
		},
	}
}

func newShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a task",
		Long: `Print one task in full.

` + idGrammarHelp + `

The human form is a header line with a status glyph, the ID, and the
title (plus ↑ when the task promotes), then one row per field: project,
area, note, due on (red, when color is on, once the date is today or
earlier), defer until, defer stage, promotes, done at, cancelled at,
status, position, created at, updated at, and tags. An empty field shows
only its label, and a multi-line note is indented under its label. With
--json the task row is written: id, project_id, area_id, title, note,
defer_until, due_on, done_at, cancelled_at, status, position,
created_at, updated_at, defer_stage_id, promotes, and tags. Timestamps
are UTC with millisecond precision, dates are YYYY-MM-DD, absent values
are null, and tags is always an array.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				found, err := application.Show(command.Context(), id)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, found, humanOutput.writeTask)
			})
		},
	}
}

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
without one the command fails with invalid_argument (exit 1) before the
database is opened.

` + idGrammarHelp + `

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

` + noteFlagHelp + `

` + dateGrammarHelp + `

` + taskContainerHelp + `

Moving a task out of its project, whether to another project, an area,
or the inbox, clears an existing stage defer unless --defer-stage is
given in the same command; the cleared task is listed under the result.
A task that does not move keeps its position, and blockers are checked
only when it moves.

` + blockerGuidanceHelp + `

On success one line reads '~ Edited: ID  TITLE', followed by a 'Cleared
stage defer' line when one was cleared. With --json the updated task row
is written, except that a command with --project, --no-project, --area,
or --no-area writes {"task":ROW,"cleared_defers":[ROW...]} instead, with
cleared_defers empty when nothing was cleared.

` + outputContractHelp,
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
				humanOutput.writeTaskEdition,
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

func newListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(task.ListStatusOpen)
	var due bool
	var overdue bool
	var deferred bool
	var projectIDValue string
	var areaIDValue string
	var tagValue string
	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long: `List tasks with optional filters, ordered by position and then ID across
all containers.

` + statusFilterHelp + `

--project ID keeps tasks in that project, and --area ID keeps loose tasks
whose own area is ID, not tasks in the area's projects; the two cannot be
combined (invalid_argument, exit 1), and an unknown project, area, or tag
is not_found (exit 1). --tag NAME keeps tasks carrying that tag, matched
case-insensitively. --due keeps tasks with a due date, --overdue keeps
open tasks due before today, and --deferred keeps tasks whose defer date
is after today or whose stage defer has not been reached; those three
are mutually exclusive (usage error, exit 2). Filters combine with AND.

The human table has id, title, status, and dates columns, with title and
dates as in 'gsd inbox'; an empty result prints nothing. With --json an
array of task rows is written (fields as in 'gsd show --help'), [] when
empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := task.ParseListStatus(statusValue)
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

			selector := task.DateSelectorNone
			if due {
				selector = task.DateSelectorDue
			}
			if overdue {
				selector = task.DateSelectorOverdue
			}
			if deferred {
				selector = task.DateSelectorDeferred
			}
			listOptions := task.ListOptions{Status: status, Date: selector, ProjectID: projectID, AreaID: areaID}
			if command.Flags().Changed("tag") {
				listOptions.Tag = &tagValue
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.List(command.Context(), listOptions)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, tasks, humanOutput.writeTaskList)
			})
		},
	}
	command.Flags().StringVar(&statusValue, "status", statusValue, "open, done, cancelled, or all")
	command.Flags().StringVar(&projectIDValue, "project", "", "filter by project ID")
	command.Flags().StringVar(&areaIDValue, "area", "", "filter by area ID")
	command.Flags().StringVar(&tagValue, "tag", "", "filter by tag")
	command.Flags().BoolVar(&due, "due", false, "list tasks with due dates")
	command.Flags().BoolVar(&overdue, "overdue", false, "list overdue open tasks")
	command.Flags().BoolVar(&deferred, "deferred", false, "list tasks deferred beyond today")
	command.MarkFlagsMutuallyExclusive("due", "overdue", "deferred")

	return command
}

func newDoneCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "done ID",
		Short: "Complete a task",
		Long: `Mark an open task done, recording done_at.

` + idGrammarHelp + `

A task that is already done or cancelled is a conflict (exit 1). When the
task promotes and its project is on a board, the project moves to the
next stage of that board in the same transaction and is placed last
there; a project already at the last stage stays where it is and the
result says so.

` + blockerGuidanceHelp + `

On success one line reads '✓ Done: ID  TITLE', followed by '~ Promoted:
◆ PID  PTITLE → STAGE' when the project moved, or that line ending in
'(already at last stage)'. With --json the task row is written, except
that a promoting task writes {"task":ROW,"promoted_project":PROJECT},
where promoted_project is null when no stage changed.

` + outputContractHelp,
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
				humanOutput.writeTaskCompletion,
			)
		},
	}
}

func newCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Mark an open task cancelled, recording cancelled_at.

` + idGrammarHelp + `

A task that is already done or cancelled is a conflict (exit 1).

` + blockerGuidanceHelp + `

On success one line reads '✗ Cancelled: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + outputContractHelp,
			short: "Cancel a task",
			use:   "cancel ID",
			verb:  verbCancelled,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Cancel(ctx, id)
		},
	)
}

func newReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Return a done or cancelled task to open, clearing done_at and
cancelled_at.

` + idGrammarHelp + `

A task that is already open is a conflict (exit 1).

` + blockerGuidanceHelp + `

On success one line reads '~ Reopened: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + outputContractHelp,
			short: "Reopen a task",
			use:   "reopen ID",
			verb:  verbReopened,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Reopen(ctx, id)
		},
	)
}

func newReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := reorderFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder a task",
		Long: `Move a task to a new position among its siblings, the tasks in the same
container: the inbox, one project, or one area. One placement flag is
required (usage error, exit 2, when none is given).

` + idGrammarHelp + `

` + placementHelp + `

On success one line reads '~ Reordered: ID  TITLE'; with --json the
updated task row is written (fields as in 'gsd show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, task.ParseID)
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, taskMutationWriter(verbReordered))
			})
		},
	}
	flags.register(command, "task")

	return command
}

func newTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to task ID.

` + taggingContractHelp,
			short: "Tag a task",
			use:   "tag ID NAME...",
			verb:  verbTagged,
		},
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from task ID.

` + taggingContractHelp,
			short: "Untag a task",
			use:   "untag ID NAME...",
			verb:  verbUntagged,
		},
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

func newDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Delete a task permanently, whatever its status and wherever it lives; a
resolved project or an archived area does not block deletion.

` + idGrammarHelp + `

On success one line reads '− Deleted: ID  TITLE'; with --json the deleted
task row is written (fields as in 'gsd show --help').

` + outputContractHelp,
			short: "Delete a task",
			use:   "delete ID",
			verb:  verbDeleted,
		},
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Delete(ctx, id)
		},
	)
}

func rejectFalseBooleanFlags(command *cobra.Command, names ...string) error {
	for _, name := range names {
		if !command.Flags().Changed(name) {
			continue
		}
		value, err := command.Flags().GetBool(name)
		if err != nil {
			return usageError("read --" + name + ": " + err.Error())
		}
		if !value {
			return usageError("--" + name + " cannot be false")
		}
	}
	return nil
}

func anyFlagChanged(command *cobra.Command, names ...string) bool {
	for _, name := range names {
		if command.Flags().Changed(name) {
			return true
		}
	}

	return false
}

func parseProjectIDFlag(command *cobra.Command, value string) (*int64, error) {
	if !command.Flags().Changed("project") {
		return nil, nil
	}

	id, err := project.ParseID(value)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func parseAreaIDFlag(command *cobra.Command, value string) (*int64, error) {
	if !command.Flags().Changed("area") {
		return nil, nil
	}

	id, err := area.ParseID(value)
	if err != nil {
		return nil, err
	}

	return &id, nil
}

func resolveNote(command *cobra.Command, value string) (string, error) {
	if value != "-" {
		return value, nil
	}

	contents, err := io.ReadAll(command.InOrStdin())
	if err != nil {
		return "", apperr.New(
			apperr.Internal,
			fmt.Sprintf("read note: %v", err),
			err,
		)
	}

	return string(contents), nil
}

type taskTaggingMutation func(context.Context, task.Application, int64, []string) (task.Tagging, error)

func newTaskTaggingCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	mutate taskTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}

			return withTaskOutput(command, options, factory,
				func(application task.Application) (task.Tagging, error) {
					return mutate(command.Context(), application, id, args[1:])
				},
				func(tagging task.Tagging) any { return tagging.Task },
				func(output humanOutput, tagging task.Tagging) error {
					return output.writeTaskTagging(spec.verb, tagging)
				},
			)
		},
	}
}

type taskMutation func(context.Context, task.Application, int64) (task.Task, error)

func newTaskMutationCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	mutate taskMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				affected, err := mutate(command.Context(), application, id)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, affected, taskMutationWriter(spec.verb))
			})
		},
	}
}
