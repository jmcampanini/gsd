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
		Args:  cobra.ExactArgs(1),
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
				Promotes:  promotes && !noPromotes,
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
			if noDeferStage {
				fields.DeferStage = nil
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
		Args:  cobra.NoArgs,
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
		Args:  cobra.NoArgs,
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				edited, editErr := application.Edit(command.Context(), id, fields)
				if editErr != nil {
					return editErr
				}
				if options.json {
					if containmentEdit {
						return writeJSON(command.OutOrStdout(), edited)
					}
					return writeJSON(command.OutOrStdout(), edited.Task)
				}
				return options.presentation.output(command).writeTaskEdition(edited)
			})
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
		Args:  cobra.NoArgs,
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
	command.Flags().StringVar(&statusValue, "status", statusValue, "filter by status: open, done, cancelled, or all")
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
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}
			return withTaskApplication(command, options, factory, func(application task.Application) error {
				completion, err := application.Done(command.Context(), id)
				if err != nil {
					return err
				}
				if options.json {
					if completion.Task.Promotes {
						return writeJSON(command.OutOrStdout(), completion)
					}
					return writeJSON(command.OutOrStdout(), completion.Task)
				}
				return options.presentation.output(command).writeTaskCompletion(completion)
			})
		},
	}
}

func newCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"cancel ID",
		"Cancel a task",
		verbCancelled,
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Cancel(ctx, id)
		},
	)
}

func newReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"reopen ID",
		"Reopen a task",
		verbReopened,
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
		Args:  cobra.ExactArgs(1),
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
		"tag ID NAME...",
		"Tag a task",
		verbTagged,
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskTaggingCommand(
		options,
		factory,
		"untag ID NAME...",
		"Untag a task",
		verbUntagged,
		func(ctx context.Context, application task.Application, id int64, names []string) (task.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

func newDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"delete ID",
		"Delete a task",
		verbDeleted,
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
	use string,
	short string,
	verb mutationVerb,
	mutate taskTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := task.ParseID(args[0])
			if err != nil {
				return err
			}

			return withTaskApplication(command, options, factory, func(application task.Application) error {
				tagging, err := mutate(command.Context(), application, id, args[1:])
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), tagging.Task)
				}

				return options.presentation.output(command).writeTaskTagging(verb, tagging)
			})
		},
	}
}

type taskMutation func(context.Context, task.Application, int64) (task.Task, error)

func newTaskMutationCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	verb mutationVerb,
	mutate taskMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
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
				return writeCommandOutput(command, options, affected, taskMutationWriter(verb))
			})
		},
	}
}
