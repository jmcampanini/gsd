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
		return apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid placement anchor %q", placement.Anchor),
			nil,
		)
	}

	return nil
}
