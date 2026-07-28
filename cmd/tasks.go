package cmd

import (
	"context"

	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

func newAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a task to the inbox",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withApplication(command, options, factory, func(application task.Application) error {
				created, err := application.Add(command.Context(), args[0], note)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), created)
				}

				return writeAddedTask(command.OutOrStdout(), created)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "task note")

	return command
}

func newInboxCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "inbox",
		Short: "List open inbox tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.Inbox(command.Context())
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), tasks)
				}

				return writeInbox(command.OutOrStdout(), tasks)
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

			return withApplication(command, options, factory, func(application task.Application) error {
				found, err := application.Show(command.Context(), id)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), found)
				}

				return writeTask(command.OutOrStdout(), found)
			})
		},
	}
}

func newListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(task.ListStatusOpen)
	command := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := task.ParseListStatus(statusValue)
			if err != nil {
				return err
			}

			return withApplication(command, options, factory, func(application task.Application) error {
				tasks, err := application.List(command.Context(), status)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), tasks)
				}

				return writeTaskList(command.OutOrStdout(), tasks)
			})
		},
	}
	command.Flags().StringVar(&statusValue, "status", statusValue, "filter by status: open, done, cancelled, or all")

	return command
}

func newDoneCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"done ID",
		"Complete a task",
		"Done",
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Done(ctx, id)
		},
	)
}

func newCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"cancel ID",
		"Cancel a task",
		"Cancelled",
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
		"Reopened",
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Reopen(ctx, id)
		},
	)
}

func newDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newTaskMutationCommand(
		options,
		factory,
		"delete ID",
		"Delete a task",
		"Deleted",
		func(ctx context.Context, application task.Application, id int64) (task.Task, error) {
			return application.Delete(ctx, id)
		},
	)
}

type taskMutation func(context.Context, task.Application, int64) (task.Task, error)

func newTaskMutationCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	action string,
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

			return withApplication(command, options, factory, func(application task.Application) error {
				affected, err := mutate(command.Context(), application, id)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), affected)
				}

				return writeTaskMutation(command.OutOrStdout(), action, affected)
			})
		},
	}
}
