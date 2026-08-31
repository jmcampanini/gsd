package cmd

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/spf13/cobra"
)

func newBoardsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("boards", "Manage boards", `Manage boards as a set: 'boards add NAME' creates one and 'boards list'
lists them. 'gsd board' holds the commands that act on one existing
board, and 'gsd stages' and 'gsd stage' manage the stages on a board.

A board is an ordered list of stages, addressed by name. A project can
sit in one stage of one board and moves through the stages with 'gsd
project move' or by promotion when a promoting task in it is completed.
Stage order also decides when a task's stage defer counts as reached.
Board titles are unique case-insensitively, and so are stage titles
within one board.

`+groupContractHelp)
	command.AddCommand(
		newBoardsAddCommand(options, factory),
		newBoardsListCommand(options, factory),
	)
	return command
}

func newBoardsAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var stages []string
	var note string
	command := &cobra.Command{
		Use:   "add NAME",
		Short: "Add a board",
		Long: `Create a board titled NAME with the stages given by --stage, in that
order, and print it. NAME and every stage name must not be blank, and at
least one --stage is required (invalid_argument, exit 1). A board whose
title matches NAME case-insensitively is a conflict (exit 1), as is a
--stage name repeated within the command; then nothing is created. The
board is placed last among all boards.

` + noteFlagHelp + `

On success one line reads '+ Board: NAME (STAGE → STAGE ...)'; with
--json the new board row is written (fields as in 'gsd board show
--help') without its stages, which 'gsd board show NAME' lists.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedNote, err := resolveNote(command, note)
			if err != nil {
				return err
			}
			fields := board.AddFields{Title: args[0], Note: resolvedNote, Stages: stages}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.Addition, error) {
					return application.Add(command.Context(), fields)
				},
				func(addition board.Addition) any { return addition.Board },
				humanOutput.writeAddedBoard,
			)
		},
	}
	command.Flags().StringArrayVar(&stages, "stage", nil, "stage name (repeatable)")
	command.Flags().StringVar(&note, "note", "", "board note or - to read stdin")
	return command
}

func newBoardsListCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List boards",
		Long: `List every board, ordered by position and then ID, with its stages in
order.

The human table has board and stages columns, the stages joined by →; an
empty result prints nothing. With --json an array is written, each
element a board row (fields as in 'gsd board show --help') plus
"stages", an array of stage rows in order; [] when empty.

` + outputContractHelp,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				listed, err := application.List(command.Context())
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, listed, humanOutput.writeBoardList)
			})
		},
	}
}

func newBoardCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("board", "Manage a board", `Act on one existing board by NAME: show, edit, reorder, and delete. New
boards come from 'gsd boards add' and lists from 'gsd boards list';
'gsd boards --help' describes what a board is, and 'gsd stages' and 'gsd
stage' manage a board's stages.

`+nameGrammarHelp+`

`+groupContractHelp)
	command.AddCommand(
		newBoardShowCommand(options, factory),
		newBoardEditCommand(options, factory),
		newBoardReorderCommand(options, factory),
		newBoardDeleteCommand(options, factory),
	)
	return command
}

func newBoardShowCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "show NAME",
		Short: "Show a board",
		Long: `Print a board with its stages and the open projects in each stage.

` + nameGrammarHelp + `

The human form is a headline 'NAME  STAGE → STAGE ...' (or '(no
stages)'), then one line per open project under its stage, '◆ ID  TITLE
DONE/TOTAL', where DONE counts the project's done tasks and TOTAL its
tasks that are not cancelled, or '(empty)' for a stage without open
projects. Resolved projects are not shown. With --json
{"board":ROW,"stages":[...]} is written, where the board row has id,
title, note, position, created_at, and updated_at, and each stage has
id, board_id, title, position, created_at, updated_at, and "projects",
an array of project rows (fields as in 'gsd project show --help') each
carrying "progress":{"done":N,"total":N}.

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				shown, err := application.Show(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, shown, humanOutput.writeBoard)
			})
		},
	}
}

func newBoardEditCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	var title string
	var note string
	command := &cobra.Command{
		Use:   "edit NAME",
		Short: "Edit a board",
		Long: `Change the title or note of a board. At least one of --title and --note
is required; without one the command fails with invalid_argument (exit
1) before the database is opened. A new title already used by another
board, compared case-insensitively, is a conflict (exit 1).

` + nameGrammarHelp + `

` + noteFlagHelp + `

On success one line reads '~ Edited: board TITLE'; with --json the
updated board row is written (fields as in 'gsd board show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !anyFlagChanged(command, "title", "note") {
				return apperr.New(
					apperr.InvalidArgument,
					"board edit requires --title or --note",
					nil,
				)
			}

			fields := board.EditFields{}
			if command.Flags().Changed("title") {
				fields.Title = &title
			}
			if command.Flags().Changed("note") {
				resolvedNote, err := resolveNote(command, note)
				if err != nil {
					return err
				}
				fields.Note = &resolvedNote
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				edited, err := application.Edit(command.Context(), args[0], fields)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, edited, boardMutationWriter(verbEdited))
			})
		},
	}
	command.Flags().StringVar(&title, "title", "", "board title")
	command.Flags().StringVar(&note, "note", "", "board note or - to read stdin")
	return command
}

func newBoardReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "reorder NAME",
		Short: "Reorder a board",
		Long: `Move a board to a new position among all boards, the order 'gsd boards
list' shows. One placement flag is required (usage error, exit 2, when
none is given).

` + nameGrammarHelp + `

` + namedPlacementHelp + `

On success one line reads '~ Reordered: board NAME'; with --json the
updated board row is written (fields as in 'gsd board show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.placement(command)
			if err != nil {
				return err
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				reordered, err := application.Reorder(command.Context(), args[0], placement)
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, reordered, boardMutationWriter(verbReordered))
			})
		},
	}
	flags.register(command, "board", true)
	return command
}

func newBoardDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a board",
		Long: `Delete a board and all of its stages. A board with any project on it,
open or resolved, is a conflict (exit 1) whose message counts them; move
those projects off with 'gsd project edit ID --no-board' or onto another
board first.

` + nameGrammarHelp + `

On success one line reads '− Deleted: board NAME'; with --json
{"board":ROW,"stages":[ROW...]} is written with the deleted board and
its stages (fields as in 'gsd board show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				deletion, err := application.Delete(command.Context(), args[0])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, deletion, humanOutput.writeBoardDeletion)
			})
		},
	}
}

func newStagesCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("stages", "Manage stages", `Add stages to a board with 'stages add BOARD NAME'. 'gsd stage' holds the
commands that act on one existing stage, and 'gsd boards --help'
describes boards and stages.

`+groupContractHelp)
	command.AddCommand(newStagesAddCommand(options, factory))
	return command
}

func newStagesAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "add BOARD NAME",
		Short: "Add a stage",
		Long: `Add stage NAME to board BOARD and print it. The stage goes last unless a
placement flag says otherwise. A stage on that board whose title matches
NAME case-insensitively is a conflict (exit 1).

` + nameGrammarHelp + `

Placement is optional here, and the siblings are BOARD's stages.

` + namedPlacementHelp + `

On success one line reads '+ Added stage BOARD/NAME'; with --json the new
stage row is written (fields as in 'gsd board show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.optionalPlacement(command)
			if err != nil {
				return err
			}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageResult, error) {
					return application.AddStage(command.Context(), args[0], args[1], placement)
				},
				func(result board.StageResult) any { return result.Stage },
				humanOutput.writeAddedStage,
			)
		},
	}
	flags.register(command, "stage", false)
	return command
}

func newStageCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("stage", "Manage a stage", `Act on one existing stage, named by its board and its own name: rename,
reorder, and delete. New stages come from 'gsd stages add', and 'gsd
boards --help' describes boards and stages.

`+groupContractHelp)
	command.AddCommand(
		newStageRenameCommand(options, factory),
		newStageReorderCommand(options, factory),
		newStageDeleteCommand(options, factory),
	)
	return command
}

func newStageRenameCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "rename BOARD OLD NEW",
		Short: "Rename a stage",
		Long: `Rename stage OLD on board BOARD to NEW. NEW must not be blank, and a
different stage on the board whose title matches NEW case-insensitively
is a conflict (exit 1); changing only the letter case of OLD is allowed.
Projects in the stage and task defers pointing at it follow the rename.

` + nameGrammarHelp + `

On success one line reads '~ Renamed stage BOARD/OLD to BOARD/NEW'; with
--json the updated stage row is written (fields as in 'gsd board show
--help').

` + outputContractHelp,
		Args: cobra.ExactArgs(3),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageRenameResult, error) {
					return application.RenameStage(command.Context(), args[0], args[1], args[2])
				},
				func(renaming board.StageRenameResult) any { return renaming.Stage },
				humanOutput.writeRenamedStage,
			)
		},
	}
}

func newStageReorderCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "reorder BOARD NAME",
		Short: "Reorder a stage",
		Long: `Move stage NAME to a new position among the stages of board BOARD. One
placement flag is required (usage error, exit 2, when none is given).
Stage order decides the next stage for promotion and whether a task's
stage defer counts as reached, so reordering can change 'gsd available'.

` + nameGrammarHelp + `

` + namedPlacementHelp + `

On success one line reads '~ Reordered: stage BOARD/NAME'; with --json
the updated stage row is written (fields as in 'gsd board show --help').

` + outputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			placement, err := flags.placement(command)
			if err != nil {
				return err
			}
			return withBoardOutput(command, options, factory,
				func(application board.Application) (board.StageResult, error) {
					return application.ReorderStage(command.Context(), args[0], args[1], placement)
				},
				func(result board.StageResult) any { return result.Stage },
				stageMutationWriter(verbReordered),
			)
		},
	}
	flags.register(command, "stage", true)
	return command
}

func newStageDeleteCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "delete BOARD NAME",
		Short: "Delete a stage",
		Long: `Delete stage NAME from board BOARD. A stage with any project in it, open
or resolved, is a conflict (exit 1) whose message counts them. Task stage
defers pointing at the stage are cleared in the same transaction and the
cleared tasks are listed.

` + nameGrammarHelp + `

On success one line reads '− Deleted: stage BOARD/NAME', followed by a
'Cleared stage defer' line per task. With --json
{"stage":ROW,"cleared_defers":[ROW...]} is written, with cleared_defers
empty when no task deferred to the stage.

` + outputContractHelp,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				result, err := application.DeleteStage(command.Context(), args[0], args[1])
				if err != nil {
					return err
				}
				return writeCommandOutput(command, options, result, humanOutput.writeStageDeletion)
			})
		},
	}
}

func newBoardCommandGroup(use, short, long string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return usageError(use + " requires a subcommand")
		},
	}
}

type namedPlacementFlags struct {
	after  string
	before string
	first  bool
	last   bool
}

func (f *namedPlacementFlags) register(command *cobra.Command, entity string, required bool) {
	command.Flags().StringVar(&f.after, "after", "", "place after "+entity+" name")
	command.Flags().StringVar(&f.before, "before", "", "place before "+entity+" name")
	command.Flags().BoolVar(&f.first, "first", false, "place first")
	command.Flags().BoolVar(&f.last, "last", false, "place last")
	command.MarkFlagsMutuallyExclusive("after", "before", "first", "last")
	if required {
		command.MarkFlagsOneRequired("after", "before", "first", "last")
	}
}

func (f namedPlacementFlags) placement(command *cobra.Command) (board.Placement, error) {
	if err := rejectFalseBooleanFlags(command, "first", "last"); err != nil {
		return board.Placement{}, err
	}

	switch {
	case command.Flags().Changed("after"):
		return board.Placement{Anchor: domain.PlacementAfter, Reference: f.after}, nil
	case command.Flags().Changed("before"):
		return board.Placement{Anchor: domain.PlacementBefore, Reference: f.before}, nil
	case command.Flags().Changed("first"):
		return board.Placement{Anchor: domain.PlacementFirst}, nil
	default:
		return board.Placement{Anchor: domain.PlacementLast}, nil
	}
}

func (f namedPlacementFlags) optionalPlacement(command *cobra.Command) (*board.Placement, error) {
	if !anyFlagChanged(command, "after", "before", "first", "last") {
		return nil, nil
	}
	placement, err := f.placement(command)
	if err != nil {
		return nil, err
	}
	return &placement, nil
}
