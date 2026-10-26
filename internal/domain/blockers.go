package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ResolvedProjectsError reports the done or cancelled projects that block an
// operation on their contents.
type ResolvedProjectsError struct {
	IDs []int64
}

func (e ResolvedProjectsError) Error() string {
	return fmt.Sprintf("resolved projects block this operation: %v", e.IDs)
}

// ArchivedAreasError reports the archived areas that block an operation on
// their contents.
type ArchivedAreasError struct {
	IDs []int64
}

func (e ArchivedAreasError) Error() string {
	return fmt.Sprintf("archived areas block this operation: %v", e.IDs)
}

// SortedUniqueIDs returns a sorted copy of ids with duplicates removed; the
// input is not modified.
func SortedUniqueIDs(ids []int64) []int64 {
	normalized := append([]int64(nil), ids...)
	slices.Sort(normalized)

	return slices.Compact(normalized)
}

// FormatIDs joins ids as decimals separated by ", " for error messages.
func FormatIDs(ids []int64) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = strconv.FormatInt(id, 10)
	}

	return strings.Join(values, ", ")
}

// SameOptionalID reports whether two optional IDs are both unset or both set
// to the same value.
func SameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
