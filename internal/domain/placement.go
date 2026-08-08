package domain

import (
	"fmt"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type PlacementAnchor string

const (
	PlacementFirst  PlacementAnchor = "first"
	PlacementLast   PlacementAnchor = "last"
	PlacementAfter  PlacementAnchor = "after"
	PlacementBefore PlacementAnchor = "before"
)

type Placement struct {
	Anchor      PlacementAnchor
	ReferenceID int64
}

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
