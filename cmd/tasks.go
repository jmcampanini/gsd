package cmd

import (
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
