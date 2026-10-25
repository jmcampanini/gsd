package cli

import (
	"github.com/jmcampanini/gsd/internal/board"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/spf13/cobra"
)

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
