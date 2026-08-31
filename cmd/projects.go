package cmd

import (
	"context"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/spf13/cobra"
)

func newProjectsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "projects",
		Short: "Manage projects",
		Long: `Manage projects as a set: 'projects add TITLE' creates one and 'projects
list' lists them. 'gsd project' holds the commands that act on one
existing project.

A project groups tasks. It has a title, a note, tags, a status of open,
done, or cancelled, and a position among the projects filed under the
same area (or among those with no area). It may sit on one board, in one
stage of that board, where it has a separate position among the projects
in that stage. Resolving a project cancels its open tasks, and a resolved
project blocks changes to its tasks until it is reopened.

` + groupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError("projects requires a subcommand")
		},
	}
	command.AddCommand(
		newProjectsAddCommand(options, factory),
		newProjectsListCommand(options, factory),
	)

	return command
}

func newProjectsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var note string
	var areaIDValue string
	var boardTitle string
	var tags []string
	command := &cobra.Command{
		Use:   "add TITLE",
		Short: "Add a project",
		Long: `Create an open project titled TITLE and print it. TITLE must not be
blank.

--area ID files the project under an area; an unknown area is not_found
and an archived one is a conflict (both exit 1). --board NAME puts the
project on a board, in its first stage and last among the projects
there; an unknown board is not_found and a board without stages is a
conflict (both exit 1). The project is placed last among the projects
filed under its area, or among those with no area.

` + noteFlagHelp + `

` + tagFlagHelp + `

` + blockerGuidanceHelp + `

On success one line reads '+ Added project ID: TITLE' followed by any
tags; with --json the new project row is written (fields as in 'gsd
project show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}

			fields := project.AddRequest{AreaID: areaID, Title: args[0], Note: resolvedNote, Tags: tags}
			if command.Flags().Changed("board") {
				fields.Board = &boardTitle
			}
			return withProjectApplication(command, options, factory, func(application project.Application) error {
				created, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				return writeCommandOutput(command, options, created, humanOutput.writeAddedProject)
			})
		},
	}
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().StringVar(&boardTitle, "board", "", "board name")
	command.Flags().StringArrayVar(&tags, "tag", nil, "tag name to attach (repeatable)")

	return command
}

func newProjectsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	statusValue := string(project.ListStatusOpen)
	var areaIDValue string
	command := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Long: `List projects, ordered by position and then ID across all areas.

` + statusFilterHelp + `

--area ID keeps projects filed under that area; an unknown area is
not_found (exit 1).

The human table has id, title, and status columns; an empty result
prints nothing. With --json an array of project rows is written (fields
as in 'gsd project show --help'), [] when empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			status, err := project.ParseListStatus(statusValue)
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				projects, listErr := application.List(command.Context(), project.ListOptions{Status: status, AreaID: areaID})
				if listErr != nil {
					return listErr
				}
				return writeCommandOutput(command, options, projects, humanOutput.writeProjectList)
			})
		},
	}
	command.Flags().StringVar(
		&statusValue,
		"status",
		statusValue,
		"open, done, cancelled, or all",
	)
	command.Flags().StringVar(&areaIDValue, "area", "", "filter by area ID")

	return command
}

func newProjectCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := &cobra.Command{
		Use:   "project",
		Short: "Manage a project",
		Long: `Act on one existing project by ID: show, edit, tag, untag, reorder, move
(between the stages of its board), done, cancel, reopen, and delete. New
projects come from 'gsd projects add' and lists from 'gsd projects list';
'gsd projects --help' describes what a project is.

` + groupContractHelp,
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError("project requires a subcommand")
		},
	}
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

func newProjectShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a project",
		Long: `Print one project in full.

` + idGrammarHelp + `

The human form is a header line with a status glyph, the ID, and the
title, then one row per field: area, board (as BOARD/STAGE), note, done
at, cancelled at, status, position, created at, updated at, and tags. An
empty field shows only its label, and a multi-line note is indented
under its label. With --json the project row is written: id, area_id,
title, note, done_at, cancelled_at, status, position, created_at,
updated_at, stage_id, stage_position, and tags; the board and stage
appear only as stage_id and stage_position. Timestamps are UTC with
millisecond precision, absent values are null, and tags is always an
array.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Detail, error) {
					return application.Show(command.Context(), id)
				},
				func(found project.Detail) any { return found.Project },
				humanOutput.writeProject,
			)
		},
	}
}

type projectTaggingMutation func(context.Context, project.Application, int64, []string) (project.Tagging, error)

func newProjectTagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Attach each NAME to project ID.

` + taggingContractHelp,
			short: "Tag a project",
			use:   "tag ID NAME...",
			verb:  verbTagged,
		},
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Tag(ctx, id, names)
		},
	)
}

func newProjectUntagCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectTaggingCommand(
		options,
		factory,
		commandSpec{
			long: `Detach each NAME from project ID.

` + taggingContractHelp,
			short: "Untag a project",
			use:   "untag ID NAME...",
			verb:  verbUntagged,
		},
		func(ctx context.Context, application project.Application, id int64, names []string) (project.Tagging, error) {
			return application.Untag(ctx, id, names)
		},
	)
}

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
				func(output humanOutput, tagging project.Tagging) error {
					return output.writeProjectTagging(spec.verb, tagging)
				},
			)
		},
	}
}

func newProjectDoneCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectResolveCommand(
		options,
		factory,
		commandSpec{
			long: `Mark an open project done, recording done_at, and cancel every open task
in it in the same transaction.

` + idGrammarHelp + `

A project that is already done or cancelled is a conflict (exit 1), as
is one filed under an archived area.

` + blockerGuidanceHelp + `

On success one line reads '✓ Done: project ID  TITLE', followed by
'Cancelled N open tasks:' and one line per task when any were open. With
--json {"project":ROW,"cancelled_tasks":[ROW...]} is written, with
cancelled_tasks empty when none were open.

` + outputContractHelp,
			short: "Complete a project",
			use:   "done ID",
			verb:  verbDone,
		},
		project.ExitDone,
	)
}

func newProjectCancelCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return newProjectResolveCommand(
		options,
		factory,
		commandSpec{
			long: `Mark an open project cancelled, recording cancelled_at, and cancel every
open task in it in the same transaction.

` + idGrammarHelp + `

A project that is already done or cancelled is a conflict (exit 1), as
is one filed under an archived area.

` + blockerGuidanceHelp + `

On success one line reads '✗ Cancelled: project ID  TITLE', followed by
'Cancelled N open tasks:' and one line per task when any were open. With
--json {"project":ROW,"cancelled_tasks":[ROW...]} is written, with
cancelled_tasks empty when none were open.

` + outputContractHelp,
			short: "Cancel a project",
			use:   "cancel ID",
			verb:  verbCancelled,
		},
		project.ExitCancelled,
	)
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
				return writeCommandOutput(command, options, resolution, projectResolutionWriter(spec.verb))
			})
		},
	}
}

func newProjectMoveCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := reorderFlags{}
	command := &cobra.Command{
		Use:   "move ID STAGE",
		Short: "Move a project on its board",
		Long: `Move a project to stage STAGE of the board it is on, placing it last in
that stage unless a placement flag says otherwise. STAGE matches
case-insensitively; a stage that is not on the project's board is
not_found (exit 1). A project that is not on a board is a conflict (exit
1), and a move to the current stage without a placement flag changes
nothing. Stage defers on the project's tasks are kept.

` + idGrammarHelp + `

Placement is optional here, and the siblings are the projects in STAGE.

` + placementHelp + `

` + blockerGuidanceHelp + `

On success one line reads '~ Moved: ◆ ID  TITLE → STAGE'; with --json the
updated project row is written (fields as in 'gsd project show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.optionalPlacement(command, project.ParseID)
			if err != nil {
				return err
			}

			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Movement, error) {
					return application.Move(command.Context(), id, args[1], placement)
				},
				func(moved project.Movement) any { return moved.Project },
				humanOutput.writeProjectMovement,
			)
		},
	}
	flags.registerOptional(command, "project")
	return command
}

func newProjectReopenCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "reopen ID",
		Short: "Reopen a project",
		Long: `Return a done or cancelled project to open, clearing done_at and
cancelled_at. Tasks cancelled when the project was resolved stay
cancelled; reopen them one at a time with 'gsd reopen ID'.

` + idGrammarHelp + `

A project that is already open is a conflict (exit 1), as is one filed
under an archived area.

` + blockerGuidanceHelp + `

On success one line reads '~ Reopened: project ID  TITLE'; with --json
the updated project row is written (fields as in 'gsd project show
--help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				reopened, err := application.Reopen(command.Context(), id)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, reopened, projectMutationWriter(verbReopened))
			})
		},
	}
}

func newProjectReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := reorderFlags{}
	command := &cobra.Command{
		Use:   "reorder ID",
		Short: "Reorder a project",
		Long: `Move a project to a new position among its siblings, the projects filed
under the same area or, for a project without one, those with no area.
This is the order 'gsd projects list' shows; the position within a board
stage is separate and changed by 'gsd project move'. One placement flag
is required (usage error, exit 2, when none is given).

` + idGrammarHelp + `

` + placementHelp + `

On success one line reads '~ Reordered: project ID  TITLE'; with --json
the updated project row is written (fields as in 'gsd project show
--help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := flags.validate(command); err != nil {
				return err
			}

			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			placement, err := flags.placement(command, project.ParseID)
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				reordered, reorderErr := application.Reorder(command.Context(), id, placement)
				if reorderErr != nil {
					return reorderErr
				}
				return writeCommandOutput(command, options, reordered, projectMutationWriter(verbReordered))
			})
		},
	}
	flags.register(command, "project")

	return command
}

func newProjectDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var recursive bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a project",
		Long: `Delete a project permanently, whatever its status or board. Without
--recursive a project that still contains tasks is a conflict (exit 1)
whose message points to --recursive; with --recursive its tasks are
deleted first, in the same transaction.

` + idGrammarHelp + `

On success one line reads '− Deleted: project ID  TITLE', followed by
'Deleted N tasks:' and one line per task when --recursive removed any.
With --json the deleted project row is written, or with --recursive
{"project":ROW,"deleted_tasks":[ROW...]}, with deleted_tasks empty when
there were none.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}

			return withProjectApplication(command, options, factory, func(application project.Application) error {
				deletion, err := application.Delete(command.Context(), id, recursive)
				if err != nil {
					if code, ok := apperr.CodeOf(err); ok && code == apperr.Conflict && !recursive {
						return apperr.New(
							apperr.Conflict,
							err.Error()+"; use --recursive to delete the project and its tasks",
							err,
						)
					}
					return err
				}
				return renderResult(command, options, deletion,
					func(deletion project.Deletion) any {
						if recursive {
							return deletion
						}
						return deletion.Project
					},
					humanOutput.writeProjectDeletion,
				)
			})
		},
	}
	command.Flags().BoolVar(&recursive, "recursive", false, "delete contained tasks")

	return command
}

func newProjectEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	var areaIDValue string
	var noArea bool
	var boardTitle string
	var noBoard bool
	command := &cobra.Command{
		Use:   "edit ID",
		Short: "Edit a project",
		Long: `Change one or more fields of a project. At least one of --title, --note,
--area, --no-area, --board, and --no-board is required; without one the
command fails with invalid_argument (exit 1) before the database is
opened.

` + idGrammarHelp + `

--title TEXT and --note replace the title and note. --area ID and
--no-area file the project under another area or under none; an archived
source or destination area is a conflict (exit 1), while a resolved
project may be refiled. --board NAME and --no-board put the project on a
board, in its first stage and last among the projects there, or take it
off its board; a project already on NAME stays where it is. A board move
refuses a resolved project or an archived area (conflict, exit 1), needs
a board with at least one stage, and clears every stage defer on the
project's tasks, listing each cleared task. --area with --no-area and
--board with --no-board are mutually exclusive, and --no-area and
--no-board cannot be given as false (usage errors, exit 2).

` + noteFlagHelp + `

` + blockerGuidanceHelp + `

On success one line reads '~ Edited: project ID  TITLE', or with --board
or --no-board '~ Edited: ◆ ID  TITLE → BOARD/STAGE' (or '→ (no board)'),
followed by a 'Cleared stage defer' line per cleared task. With --json
the updated project row is written, except that a command with --board
or --no-board writes {"project":ROW,"cleared_defers":[ROW...]}, with
cleared_defers empty when nothing was cleared.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := rejectFalseBooleanFlags(command, "no-area", "no-board"); err != nil {
				return err
			}
			id, err := project.ParseID(args[0])
			if err != nil {
				return err
			}
			areaID, err := parseAreaIDFlag(command, areaIDValue)
			if err != nil {
				return err
			}

			if !anyFlagChanged(command, "title", "note", "area", "no-area", "board", "no-board") {
				return apperr.New(
					apperr.InvalidArgument,
					"project edit requires --title, --note, --area, --no-area, --board, or --no-board",
					nil,
				)
			}

			fields := project.EditRequest{}
			fields.Area.Set = areaID
			fields.Area.Clear = noArea
			if command.Flags().Changed("board") {
				fields.Board.Set = &boardTitle
			}
			fields.Board.Clear = noBoard
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

			boardChanged := command.Flags().Changed("board") || command.Flags().Changed("no-board")
			return withProjectOutput(command, options, factory,
				func(application project.Application) (project.Edition, error) {
					return application.Edit(command.Context(), id, fields)
				},
				func(edited project.Edition) any {
					if boardChanged {
						return edited
					}
					return edited.Project
				},
				func(output humanOutput, edited project.Edition) error {
					if boardChanged {
						return output.writeProjectBoardEdit(edited)
					}
					return output.writeProjectMutation(verbEdited, edited.Project)
				},
			)
		},
	}
	command.Flags().StringVar(&title, "title", "", "project title")
	command.Flags().StringVar(&note, "note", "", "project note or - to read stdin")
	command.Flags().StringVar(&areaIDValue, "area", "", "area ID")
	command.Flags().BoolVar(&noArea, "no-area", false, "remove the project from its area")
	command.Flags().StringVar(&boardTitle, "board", "", "board name")
	command.Flags().BoolVar(&noBoard, "no-board", false, "remove the project from its board")
	command.MarkFlagsMutuallyExclusive("area", "no-area")
	command.MarkFlagsMutuallyExclusive("board", "no-board")

	return command
}
