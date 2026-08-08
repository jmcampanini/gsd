package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type ResolvedProjectsError struct {
	IDs []int64
}

func (e ResolvedProjectsError) Error() string {
	return fmt.Sprintf("resolved projects block this operation: %v", e.IDs)
}

type ArchivedAreasError struct {
	IDs []int64
}

func (e ArchivedAreasError) Error() string {
	return fmt.Sprintf("archived areas block this operation: %v", e.IDs)
}

func SortedUniqueIDs(ids []int64) []int64 {
	normalized := append([]int64(nil), ids...)
	slices.Sort(normalized)

	return slices.Compact(normalized)
}

func FormatIDs(ids []int64) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = strconv.FormatInt(id, 10)
	}

	return strings.Join(values, ", ")
}

func SameOptionalID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}

	return *left == *right
}
