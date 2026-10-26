package domain

import (
	"fmt"

	"github.com/jmcampanini/gsd/internal/apperr"
)

// PlacementAnchor names where a reordered row lands relative to its
// siblings.
type PlacementAnchor string

// The placement anchors. PlacementFirst and PlacementLast take no reference;
// PlacementAfter and PlacementBefore require one.
const (
	PlacementFirst  PlacementAnchor = "first"
	PlacementLast   PlacementAnchor = "last"
	PlacementAfter  PlacementAnchor = "after"
	PlacementBefore PlacementAnchor = "before"
)

// Placement is an ID-addressed reorder target. ReferenceID is the sibling
// row for PlacementAfter and PlacementBefore and must be zero otherwise.
type Placement struct {
	Anchor      PlacementAnchor
	ReferenceID int64
}

// ValidatePlacement checks that the anchor is known and that ReferenceID is
// present exactly when the anchor needs one.
func ValidatePlacement(placement Placement) error {
	switch placement.Anchor {
	case PlacementFirst, PlacementLast:
		if placement.ReferenceID != 0 {
			return apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("%s placement must not include a reference ID", placement.Anchor),
				nil,
			)
		}
	case PlacementAfter, PlacementBefore:
		if placement.ReferenceID <= 0 {
			return apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("%s placement reference ID must be positive", placement.Anchor),
				nil,
			)
		}
	default:
		return invalidPlacementAnchor(placement.Anchor)
	}

	return nil
}

// ValidateNamedPlacement checks a placement whose reference is a name rather
// than an ID (boards and stages are name-addressed).
func ValidateNamedPlacement(anchor PlacementAnchor, reference string) error {
	switch anchor {
	case PlacementFirst, PlacementLast:
		if reference != "" {
			return apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("%s placement must not include a reference", anchor),
				nil,
			)
		}
	case PlacementAfter, PlacementBefore:
		if err := ValidateRequiredText("placement reference", reference); err != nil {
			return err
		}
	default:
		return invalidPlacementAnchor(anchor)
	}

	return nil
}

func invalidPlacementAnchor(anchor PlacementAnchor) error {
	return apperr.New(
		apperr.InvalidArgument,
		fmt.Sprintf("invalid placement anchor %q", anchor),
		nil,
	)
}
