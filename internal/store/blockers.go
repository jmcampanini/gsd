package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
)

func taskBlockersConflict(
	action string,
	resolvedProjectIDs []int64,
	archivedAreaIDs []int64,
	cause error,
) error {
	resolvedProjectIDs = domain.SortedUniqueIDs(resolvedProjectIDs)
	archivedAreaIDs = domain.SortedUniqueIDs(archivedAreaIDs)

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
			fmt.Sprintf("projects %s are resolved", domain.FormatIDs(resolvedProjectIDs)),
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
			fmt.Sprintf("areas %s are archived", domain.FormatIDs(archivedAreaIDs)),
		)
	}

	message := fmt.Sprintf("cannot %s while %s", action, strings.Join(blockers, " and "))
	causes := []error{cause}
	if len(resolvedProjectIDs) > 0 {
		causes = append(causes, &domain.ResolvedProjectsError{IDs: resolvedProjectIDs})
	}
	if len(archivedAreaIDs) > 0 {
		causes = append(causes, &domain.ArchivedAreasError{IDs: archivedAreaIDs})
	}

	return apperr.New(apperr.Conflict, message, errors.Join(causes...))
}
