package clierror

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
)

// NormalizeApplication preserves coded errors, adds recovery guidance, and codes unexpected errors as internal.
func NormalizeApplication(err error) error {
	if err == nil {
		return nil
	}
	if code, ok := apperr.CodeOf(err); ok {
		message := err.Error()
		guided := message
		var resolvedProjects *domain.ResolvedProjectsError
		if errors.As(err, &resolvedProjects) {
			guided = appendRecoveryGuidance(guided, "reopen", "project reopen", resolvedProjects.IDs)
		}
		var archivedAreas *domain.ArchivedAreasError
		if errors.As(err, &archivedAreas) {
			guided = appendRecoveryGuidance(guided, "unarchive", "area unarchive", archivedAreas.IDs)
		}
		if guided != message {
			return apperr.New(code, guided, err)
		}
		return err
	}

	return apperr.New(apperr.Internal, err.Error(), err)
}

func appendRecoveryGuidance(message, verb, command string, ids []int64) string {
	unique := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		unique[id] = struct{}{}
	}
	ordered := make([]int64, 0, len(unique))
	for id := range unique {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })

	commands := make([]string, 0, len(ordered))
	for _, id := range ordered {
		commands = append(commands, fmt.Sprintf("gsd %s %d", command, id))
	}
	if len(commands) == 0 {
		return message
	}

	return message + "; " + verb + " first: " + strings.Join(commands, "; ")
}

// Usage constructs an uncoded error for the root adapter to map to exit status 2.
func Usage(message string) error {
	return errors.New(message)
}
