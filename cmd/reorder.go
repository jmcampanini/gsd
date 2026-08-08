package cmd

import (
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/spf13/cobra"
)

type reorderFlags struct {
	afterIDValue  string
	beforeIDValue string
	first         bool
	last          bool
}

func (f *reorderFlags) register(command *cobra.Command, entity string) {
	f.registerFlags(command, entity)
	command.MarkFlagsOneRequired("after", "before", "first", "last")
}

func (f *reorderFlags) registerOptional(command *cobra.Command, entity string) {
	f.registerFlags(command, entity)
}

func (f *reorderFlags) registerFlags(command *cobra.Command, entity string) {
	command.Flags().StringVar(&f.afterIDValue, "after", "", "place after "+entity+" ID")
	command.Flags().StringVar(&f.beforeIDValue, "before", "", "place before "+entity+" ID")
	command.Flags().BoolVar(&f.first, "first", false, "place first")
	command.Flags().BoolVar(&f.last, "last", false, "place last")
	command.MarkFlagsMutuallyExclusive("after", "before", "first", "last")
}

func (f reorderFlags) validate(command *cobra.Command) error {
	if command.Flags().Changed("first") && !f.first {
		return usageError("--first cannot be false")
	}
	if command.Flags().Changed("last") && !f.last {
		return usageError("--last cannot be false")
	}
	return nil
}

func (f reorderFlags) optionalPlacement(
	command *cobra.Command,
	parseID func(string) (int64, error),
) (*domain.Placement, error) {
	if !anyFlagChanged(command, "after", "before", "first", "last") {
		return nil, nil
	}
	placement, err := f.placement(command, parseID)
	if err != nil {
		return nil, err
	}
	return &placement, nil
}

func (f reorderFlags) placement(
	command *cobra.Command,
	parseID func(string) (int64, error),
) (domain.Placement, error) {
	var anchor domain.PlacementAnchor
	var referenceIDValue string
	switch {
	case command.Flags().Changed("after"):
		anchor = domain.PlacementAfter
		referenceIDValue = f.afterIDValue
	case command.Flags().Changed("before"):
		anchor = domain.PlacementBefore
		referenceIDValue = f.beforeIDValue
	case command.Flags().Changed("first"):
		return domain.Placement{Anchor: domain.PlacementFirst}, nil
	case command.Flags().Changed("last"):
		return domain.Placement{Anchor: domain.PlacementLast}, nil
	default:
		return domain.Placement{}, usageError("a placement flag is required")
	}

	referenceID, err := parseID(referenceIDValue)
	if err != nil {
		return domain.Placement{}, err
	}
	return domain.Placement{Anchor: anchor, ReferenceID: referenceID}, nil
}
