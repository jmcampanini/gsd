package cmd

import (
	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/spf13/cobra"
)

func newBoardsCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	command := newBoardCommandGroup("boards", "Manage boards")
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.NoArgs,
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
	command := newBoardCommandGroup("board", "Manage a board")
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(1),
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
	command := newBoardCommandGroup("stages", "Manage stages")
	command.AddCommand(newStagesAddCommand(options, factory))
	return command
}

func newStagesAddCommand(options *rootOptions, factory applicationFactory) *cobra.Command {
	flags := namedPlacementFlags{}
	command := &cobra.Command{
		Use:   "add BOARD NAME",
		Short: "Add a stage",
		Args:  cobra.ExactArgs(2),
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
	command := newBoardCommandGroup("stage", "Manage a stage")
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
		Args:  cobra.ExactArgs(3),
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
		Args:  cobra.ExactArgs(2),
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
		Args:  cobra.ExactArgs(2),
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

func newBoardCommandGroup(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
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
