package cli

import (
	"context"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/clihelp"
	"github.com/jmcampanini/gsd/internal/render"
	"github.com/spf13/cobra"
)

func newAreaCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newCommandGroup("area", "Manage an area", `Act on one existing area by ID: show, edit, tag, untag, reorder, archive,
unarchive, and delete. New areas come from 'gsd areas add' and lists from
'gsd areas list'; 'gsd areas --help' describes what an area is.

`+clihelp.GroupContractHelp)
	command.AddCommand(
		newAreaArchiveCommand(options, factory),
		newAreaDeleteCommand(options, factory),
		newAreaEditCommand(options, factory),
		newAreaReorderCommand(options, factory),
		newAreaShowCommand(options, factory),
		newAreaTagCommand(options, factory),
		newAreaUnarchiveCommand(options, factory),
		newAreaUntagCommand(options, factory),
	)

	return command
}

type areaTaggingMutation func(context.Context, area.Application, int64, []string) (area.Tagging, error)

func newAreaTaggingCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	mutate areaTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaOutput(command, options, factory,
				func(application area.Application) (area.Tagging, error) {
					return mutate(command.Context(), application, id, args[1:])
				},
				func(tagging area.Tagging) any { return tagging.Area },
				func(output render.HumanOutput, tagging area.Tagging) error {
					return output.WriteAreaTagging(spec.verb, tagging)
				},
			)
		},
	}
}

type areaMutation func(context.Context, area.Application, int64) (area.Area, error)

func newAreaMutationCommand(
	options *rootOptions,
	factory applicationFactory,
	spec commandSpec,
	mutate areaMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Long:  spec.long,
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				affected, mutationErr := mutate(command.Context(), application, id)
				if mutationErr != nil {
					return mutationErr
				}
				return writeCommandOutput(command, options, affected, render.AreaMutationWriter(spec.verb))
			})
		},
	}
}

func withAreaOutput[T any](
	command *cobra.Command,
	options *rootOptions,
	factory applicationFactory,
	run func(area.Application) (T, error),
	jsonPayload func(T) any,
	writeHuman func(render.HumanOutput, T) error,
) error {
	return withAreaApplication(command, options, factory, func(application area.Application) error {
		result, err := run(application)
		if err != nil {
			return err
		}
		return renderResult(command, options, result, jsonPayload, writeHuman)
	})
}
