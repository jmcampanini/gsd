package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/render"
	"github.com/jmcampanini/gsd/internal/task"
	"github.com/spf13/cobra"
)

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
				func(output render.HumanOutput, tagging task.Tagging) error {
					return output.WriteTaskTagging(spec.verb, tagging)
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
				return writeCommandOutput(command, options, affected, render.TaskMutationWriter(spec.verb))
			})
		},
	}
}
