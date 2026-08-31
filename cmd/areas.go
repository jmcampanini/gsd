package cmd

import (
	"context"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/spf13/cobra"
)

func newAreasCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "areas",
		Short: "Manage areas",
		Long: `Manage areas as a set: 'areas add TITLE' creates one and 'areas list'
lists them. 'gsd area' holds the commands that act on one existing area.

An area groups projects and loose tasks. It has a title, a note, tags, a
position among all areas, and an archived state: an archived area is
hidden from 'gsd areas list' by default, its tasks leave 'gsd
available', and it blocks changes to the projects and tasks it contains
until it is unarchived.

` + groupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError("areas requires a subcommand")
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
		Long: `Create an active area titled TITLE and print it. TITLE must not be
blank. The area is placed last among all areas.

` + noteFlagHelp + `

` + tagFlagHelp + `

On success one line reads '+ Added area ID: TITLE' followed by any tags;
with --json the new area row is written (fields as in 'gsd area show
--help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
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
				return writeCommandOutput(command, options, created, humanOutput.writeAddedArea)
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
		Long: `List areas, ordered by position and then ID. Active areas are listed by
default; --archived lists only archived areas and --all lists both. The
two flags are mutually exclusive (usage error, exit 2).

The human table has id, title, and state columns, where state is
'archived' or blank; an empty result prints nothing. With --json an array
of area rows is written (fields as in 'gsd area show --help'), [] when
empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
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
				return writeCommandOutput(command, options, areas, humanOutput.writeAreaList)
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
		Long: `Act on one existing area by ID: show, edit, tag, untag, reorder, archive,
unarchive, and delete. New areas come from 'gsd areas add' and lists from
'gsd areas list'; 'gsd areas --help' describes what an area is.

` + groupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError("area requires a subcommand")
		},
	}
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

func newAreaShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show an area",
		Long: `Print one area in full.

` + idGrammarHelp + `

The human form is a header line with a state glyph, the ID, and the
title, then one row per field: note, archived at, position, created at,
updated at, and tags. An empty field shows only its label, and a
multi-line note is indented under its label. With --json the area row is
written: id, title, note, archived_at, position, created_at, updated_at,
and tags. Timestamps are UTC with millisecond precision, archived_at is
null while the area is active, and tags is always an array.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
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
				return writeCommandOutput(command, options, found, humanOutput.writeArea)
			})
		},
	}
}

type areaTaggingMutation func(context.Context, area.Application, int64, []string) (area.Tagging, error)

func newAreaTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to area ID.

` + taggingContractHelp,
			short: "Tag an area",
			use:   "tag ID NAME...",
			verb:  verbTagged,
		},
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newAreaUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from area ID.

` + taggingContractHelp,
			short: "Untag an area",
			use:   "untag ID NAME...",
			verb:  verbUntagged,
		},
		func(ctx context.Context, application area.Application, id int64, names []string) (area.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

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
				func(output humanOutput, tagging area.Tagging) error {
					return output.writeAreaTagging(spec.verb, tagging)
				},
			)
		},
	}
}

type areaMutation func(context.Context, area.Application, int64) (area.Area, error)

func newAreaArchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Archive an active area, recording archived_at. An archived area is
hidden from 'gsd areas list' by default, its tasks leave 'gsd
available', and it blocks changes to the projects and tasks it contains
until 'gsd area unarchive ID'.

` + idGrammarHelp + `

An area that is already archived is a conflict (exit 1).

On success one line reads '✗ Archived: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + outputContractHelp,
			short: "Archive an area",
			use:   "archive ID",
			verb:  verbArchived,
		},
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Archive(ctx, id)
		},
	)
}

func newAreaUnarchiveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newAreaMutationCommand(
		options,
		factory,
		commandSpec{
			long: `Return an archived area to active, clearing archived_at.

` + idGrammarHelp + `

An area that is already active is a conflict (exit 1).

On success one line reads '~ Unarchived: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + outputContractHelp,
			short: "Unarchive an area",
			use:   "unarchive ID",
			verb:  verbUnarchived,
		},
		func(ctx context.Context, application area.Application, id int64) (area.Area, error) {
			return application.Unarchive(ctx, id)
		},
	)
}

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
				return writeCommandOutput(command, options, affected, areaMutationWriter(spec.verb))
			})
		},
	}
}

func newAreaReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := reorderFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder an area",
		Long: `Move an area to a new position among all areas, the order 'gsd areas
list' shows. One placement flag is required (usage error, exit 2, when
none is given).

` + idGrammarHelp + `

` + placementHelp + `

On success one line reads '~ Reordered: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := area.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, area.ParseID)
			if err != nil {
				return err
			}

			return withAreaApplication(command, options, factory, func(application area.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, areaMutationWriter(verbReordered))
			})
		},
	}
	flags.register(command, "area")

	return command
}

func newAreaDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete an area",
		Long: `Delete an area permanently, archived or not. Without --recursive an area
that still contains projects or tasks is a conflict (exit 1) whose
message points to --recursive; with --recursive the tasks in its
projects, its projects, and its loose tasks are deleted first, in the
same transaction.

` + idGrammarHelp + `

On success one line reads '− Deleted: area ID  TITLE', followed by
'Deleted N projects:' and 'Deleted N tasks:' with one line per row when
--recursive removed any. With --json the deleted area row is written, or
with --recursive
{"area":ROW,"deleted_projects":[ROW...],"deleted_tasks":[ROW...]}, with
empty arrays when there was nothing to delete.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
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
				return renderResult(command, options, deletion,
					func(deletion area.Deletion) any {
						if recursive {
							return deletion
						}
						return deletion.Area
					},
					humanOutput.writeAreaDeletion,
				)
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
		Long: `Change the title or note of an area. At least one of --title and --note
is required; without one the command fails with invalid_argument (exit
1) before the database is opened. An archived area can be edited.

` + idGrammarHelp + `

` + noteFlagHelp + `

On success one line reads '~ Edited: area ID  TITLE'; with --json the
updated area row is written (fields as in 'gsd area show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
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
				return writeCommandOutput(command, options, edited, areaMutationWriter(verbEdited))
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "area title")
	command.Flags().StringVar(&note, "note", "", "area note or - to read stdin")

	return command
}
