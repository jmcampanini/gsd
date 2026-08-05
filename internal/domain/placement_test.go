package domain

import (
	"testing"

	"github.com/jmcampanini/gsd/internal/apperr"
)

func TestValidatePlacementAcceptsOnlyValidAnchorReferenceShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		placement Placement
		valid     bool
	}{
		{name: "first", placement: Placement{Anchor: PlacementFirst}, valid: true},
		{name: "last", placement: Placement{Anchor: PlacementLast}, valid: true},
		{name: "after", placement: Placement{Anchor: PlacementAfter, ReferenceID: 7}, valid: true},
		{name: "before", placement: Placement{Anchor: PlacementBefore, ReferenceID: 7}, valid: true},
		{name: "unknown anchor", placement: Placement{Anchor: PlacementAnchor("middle")}},
		{name: "missing after reference", placement: Placement{Anchor: PlacementAfter}},
		{name: "negative before reference", placement: Placement{Anchor: PlacementBefore, ReferenceID: -1}},
		{name: "reference on first", placement: Placement{Anchor: PlacementFirst, ReferenceID: 7}},
		{name: "reference on last", placement: Placement{Anchor: PlacementLast, ReferenceID: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePlacement(test.placement)
			if test.valid {
				if err != nil {
					t.Errorf("ValidatePlacement(%#v) error = %v", test.placement, err)
				}
				return
			}
			if code, ok := apperr.CodeOf(err); !ok || code != apperr.InvalidArgument {
				t.Errorf("ValidatePlacement(%#v) error = %v, want invalid_argument", test.placement, err)
			}
		})
	}
}
