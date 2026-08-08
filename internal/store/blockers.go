package store

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
)

func taskBlockersConflict(
	action string,
	resolvedProjectIDs []int64,
	archivedAreaIDs []int64,
	cause error,
) error {
	resolvedProjectIDs = sortedUniqueIDs(resolvedProjectIDs)
	archivedAreaIDs = sortedUniqueIDs(archivedAreaIDs)

	blockers := make([]string, 0, 2)
	switch len(resolvedProjectIDs) {
	case 0:
	case 1:
		blockers = append(
			blockers,
			fmt.Sprintf("project %d is resolved", resolvedProjectIDs[0]),
		)
	default:
		blockers = append(
			blockers,
			fmt.Sprintf("projects %s are resolved", formatIDs(resolvedProjectIDs)),
		)
	}
	switch len(archivedAreaIDs) {
	case 0:
	case 1:
		blockers = append(
			blockers,
			fmt.Sprintf("area %d is archived", archivedAreaIDs[0]),
		)
	default:
		blockers = append(
			blockers,
			fmt.Sprintf("areas %s are archived", formatIDs(archivedAreaIDs)),
		)
	}

	message := fmt.Sprintf("cannot %s while %s", action, strings.Join(blockers, " and "))
	causes := []error{cause}
	if len(resolvedProjectIDs) > 0 {
		causes = append(causes, &project.ResolvedProjectsError{IDs: resolvedProjectIDs})
	}
	if len(archivedAreaIDs) > 0 {
		causes = append(causes, &area.ArchivedAreasError{IDs: archivedAreaIDs})
	}

	return apperr.New(apperr.Conflict, message, errors.Join(causes...))
}

func sortedUniqueIDs(ids []int64) []int64 {
	normalized := append([]int64(nil), ids...)
	slices.Sort(normalized)

	return slices.Compact(normalized)
}

func formatIDs(ids []int64) string {
	values := make([]string, len(ids))
	for index, id := range ids {
		values[index] = strconv.FormatInt(id, 10)
	}

	return strings.Join(values, ", ")
}
