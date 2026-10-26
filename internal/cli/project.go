package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newProjectCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("project", "Manage a project", `Act on one existing project by ID, including moving it between the stages
of its board. New projects and lists of them come from 'gsd projects',
whose --help describes what a project is.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newProjectCancelCommand(options, factory),
		newProjectDeleteCommand(options, factory),
		newProjectDoneCommand(options, factory),
		newProjectEditCommand(options, factory),
		newProjectMoveCommand(options, factory),
		newProjectReopenCommand(options, factory),
		newProjectReorderCommand(options, factory),
		newProjectShowCommand(options, factory),
		newProjectTagCommand(options, factory),
		newProjectUntagCommand(options, factory),
	)

	return command
}

type projectTaggingMutation func(context.Context, project.Application, int64, []string) (project.Tagging, error)

func newProjectTaggingCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	mutate projectTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Tagging, error) {
					return mutate(command.Context(), application, id, args[1:])
				},
				func(tagging project.Tagging) any { return tagging.Project },
				func(output render.HumanOutput, tagging project.Tagging) error {
					return output.WriteProjectTagging(spec.verb, tagging)
				},
			)
		},
	}
}

func newProjectResolveCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	exit project.Exit,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				resolution, err := application.Resolve(command.Context(), id, exit)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, resolution, render.ProjectResolutionWriter(spec.verb))
			})
		},
	}
}

func withProjectOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(project.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(render.HumanOutput, T) error,
) error {
	return withProjectApplication(command, options, factory, func(application project.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}
