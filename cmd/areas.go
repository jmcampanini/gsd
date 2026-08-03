package cmd

import (
	"context"
	"errors"
	"io"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/spf13/cobra"
)

func newAreasCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "areas",
		Short: "Manage areas",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("areas requires a subcommand")
		},
	}
	command.AddCommand(
		newAreasAddCommand(options, factory),
		newAreasListCommand(options, factory),
	)

	return command
}

func newAreasAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := area.AddFields{Title: args[0], Note: resolvedNote, Tags: tags}
			return withAreaApplication(command, options, factory, func(application area.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, created, writeAddedArea)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")

	return command
}

func newAreasListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var archived bool
	var all bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List areas",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			slice := area.ListSliceActive
			if archived {
				slice = area.ListSliceArchived
			}
			if all {
				slice = area.ListSliceAll
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				areas, err := application.List(command.Context(), area.ListOptions{Slice: slice})
				if err != nil {
					return err
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, areas, writeAreaList)
			})
		},
	}
	command.Flags().BoolVar(&archived, "archived", false, "list archived areas")
	command.Flags().BoolVar(&all, "all", false, "list active and archived areas")
	command.MarkFlagsMutuallyExclusive("archived", "all")

	return command
}

func newAreaCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "area",
		Short: "Manage an area",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("area requires a subcommand")
		},
	}
	command.AddCommand(
		newAreaArchiveCommand(options, factory),
		newAreaDeleteCommand(options, factory),
		newAreaEditCommand(options, factory),
		newAreaShowCommand(options, factory),
		newAreaTagCommand(options, factory),
		newAreaUnarchiveCommand(options, factory),
		newAreaUntagCommand(options, factory),
	)

	return command
}

func newAreaShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				found, showErr := application.Show(command.Context(), id)
				if showErr != nil {
					return showErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, found, writeArea)
			})
		},
	}
}

type areaTaggingMutation func(context.Context, area.Application, int64, []string) (area.Tagging, error)

func newAreaTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		"tag ID NAME...",
		"Tag an area",
		"Tagged",
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newAreaUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		"untag ID NAME...",
		"Untag an area",
		"Untagged",
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

func newAreaTaggingCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	action string,
	mutate areaTaggingMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				tagging, err := mutate(command.Context(), application, id, args[1:])
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), tagging.Area)
				}

				return writeAreaTagging(command.OutOrStdout(), action, tagging)
			})
		},
	}
}

type areaMutation func(context.Context, area.Application, int64) (area.Area, error)

func newAreaArchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		"archive ID",
		"Archive an area",
		"Archived",
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Archive(ctx, id)
		},
	)
}

func newAreaUnarchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		"unarchive ID",
		"Unarchive an area",
		"Unarchived",
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Unarchive(ctx, id)
		},
	)
}

func newAreaMutationCommand(
	options *rootOptions,
	factory applicationFactory,
	use string,
	short string,
	action string,
	mutate areaMutation,
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
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
				return writeCommandOutput(
					command.OutOrStdout(),
					options.json,
					affected,
					func(writer io.Writer, current area.Area) error {
						return writeAreaMutation(writer, action, current)
					},
				)
			})
		},
	}
}

func newAreaDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				deletion, deleteErr := application.Delete(command.Context(), id, recursive)
				if deleteErr != nil {
					if code, ok := apperr.CodeOf(deleteErr); ok && code == apperr.Conflict && !recursive {
						return apperr.New(
							apperr.Conflict,
							deleteErr.Error()+"; use --recursive to delete the area and its contents",
							deleteErr,
						)
					}
					return deleteErr
				}
				if options.json {
					if recursive {
						return writeJSON(command.OutOrStdout(), deletion)
					}
					return writeJSON(command.OutOrStdout(), deletion.Area)
				}

				return writeAreaDeletion(command.OutOrStdout(), deletion)
			})
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete contained projects and tasks")

	return command
}

func newAreaEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit an area",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note") {
				return apperr.New(
					apperr.InvalidArgument,
					"area edit requires --title or --note",
					nil,
				)
			}

			fields := area.EditFields{}
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

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				edited, editErr := application.Edit(command.Context(), id, fields)
				if editErr != nil {
					return editErr
				}
				return writeCommandOutput(command.OutOrStdout(), options.json, edited, writeEditedArea)
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "area title")
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")

	return command
}
