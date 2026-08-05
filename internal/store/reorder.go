package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmcampanini/gsd/internal/domain"
)

func collectSiblingIDs(rows *sql.Rows, entity string) ([]int64, error) {
	return collectRows(rows, func(scanner rowScanner) (int64, error) {
		var id int64
		err := scanner.Scan(&id)
		return id, err
	}, "scan "+entity+" reorder sibling", "iterate "+entity+" reorder siblings")
}

func spliceOrderedIDs(
	ordered []int64,
	movedID int64,
	placement domain.Placement,
) ([]int64, error) {
	withoutMoved := make([]int64, 0, len(ordered))
	movedFound := false
	for _, id := range ordered {
		if id == movedID {
			movedFound = true
			continue
		}
		withoutMoved = append(withoutMoved, id)
	}
	if !movedFound {
		return nil, errors.New("moved ID is absent from ordered IDs")
	}

	index := 0
	switch placement.Anchor {
	case domain.PlacementFirst:
	case domain.PlacementLast:
		index = len(withoutMoved)
	case domain.PlacementBefore, domain.PlacementAfter:
		index = -1
		for current, id := range withoutMoved {
			if id == placement.ReferenceID {
				index = current
				break
			}
		}
		if index < 0 {
			return nil, errors.New("reference ID is absent from ordered IDs")
		}
		if placement.Anchor == domain.PlacementAfter {
			index++
		}
	default:
		return nil, fmt.Errorf("invalid placement anchor %q", placement.Anchor)
	}

	result := make([]int64, 0, len(ordered))
	result = append(result, withoutMoved[:index]...)
	result = append(result, movedID)
	result = append(result, withoutMoved[index:]...)
	return result, nil
}

// IDs and positions are inlined because binding them would exceed SQLite's
// bind-variable limit on containers above roughly ten thousand rows.
func reorderCaseUpdate(ordered []int64, movedID int64, timestamp string) (string, []any) {
	positionCases := make([]string, 0, len(ordered))
	identifiers := make([]string, 0, len(ordered))
	for position, id := range ordered {
		positionCases = append(positionCases, fmt.Sprintf("WHEN %d THEN %d", id, position))
		identifiers = append(identifiers, fmt.Sprintf("%d", id))
	}

	clause := "position = CASE id " + strings.Join(positionCases, " ") +
		" END, updated_at = CASE WHEN id = ? THEN ? ELSE updated_at END" +
		" WHERE id IN (" + strings.Join(identifiers, ", ") + ")"
	return clause, []any{movedID, timestamp}
}
