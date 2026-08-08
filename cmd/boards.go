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
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				addition, addErr := application.Add(command.Context(), fields)
				if addErr != nil {
					return addErr
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), addition.Board)
				}
				return options.presentation.output(command).writeAddedBoard(addition)
			})
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
			placement, err := flags.placement(command, true)
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
			placement, err := flags.placement(command, false)
			if err != nil {
				return err
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				result, err := application.AddStage(command.Context(), args[0], args[1], placement)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), result.Stage)
				}
				return options.presentation.output(command).writeAddedStage(result)
			})
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
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				renaming, err := application.RenameStage(command.Context(), args[0], args[1], args[2])
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), renaming.Stage)
				}
				return options.presentation.output(command).writeRenamedStage(renaming)
			})
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
			placement, err := flags.placement(command, true)
			if err != nil {
				return err
			}
			return withBoardApplication(command, options, factory, func(application board.Application) error {
				result, err := application.ReorderStage(command.Context(), args[0], args[1], placement)
				if err != nil {
					return err
				}
				if options.json {
					return writeJSON(command.OutOrStdout(), result.Stage)
				}
				return options.presentation.output(command).writeStageMutation(verbReordered, result)
			})
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
				if options.json {
					return writeJSON(command.OutOrStdout(), result.Stage)
				}
				return options.presentation.output(command).writeStageMutation(verbDeleted, result)
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

func (f namedPlacementFlags) placement(command *cobra.Command, required bool) (board.Placement, error) {
	if command.Flags().Changed("first") && !f.first {
		return board.Placement{}, usageError("--first cannot be false")
	}
	if command.Flags().Changed("last") && !f.last {
		return board.Placement{}, usageError("--last cannot be false")
	}

	switch {
	case command.Flags().Changed("after"):
		return board.Placement{Anchor: domain.PlacementAfter, Reference: f.after}, nil
	case command.Flags().Changed("before"):
		return board.Placement{Anchor: domain.PlacementBefore, Reference: f.before}, nil
	case command.Flags().Changed("first"):
		return board.Placement{Anchor: domain.PlacementFirst}, nil
	case command.Flags().Changed("last"):
		return board.Placement{Anchor: domain.PlacementLast}, nil
	case required:
		return board.Placement{}, usageError("a placement flag is required")
	default:
		return board.Placement{}, nil
	}
}
