package navigator

import (
	"slices"
	"strings"
	"unicode"

	"github.com/sahilm/fuzzy"
)

type rowMatch struct {
	index     int
	positions []int
}

func matchRows(pattern string, source []string) []rowMatch {
	if pattern == "" {
		return nil
	}

	caseSensitive := strings.IndexFunc(pattern, unicode.IsUpper) >= 0
	matches := fuzzy.FindNoSort(pattern, source)
	results := make([]rowMatch, 0, len(matches))
	for _, match := range matches {
		positions := match.MatchedIndexes
		if caseSensitive {
			var ok bool
			positions, ok = caseSensitiveMatchPositions(pattern, source[match.Index])
			if !ok {
				continue
			}
		}
		results = append(results, rowMatch{
			index:     match.Index,
			positions: slices.Clone(positions),
		})
	}
	return results
}

func caseSensitiveMatchPositions(pattern, candidate string) ([]int, bool) {
	patternRunes := []rune(pattern)
	positions := make([]int, 0, len(patternRunes))
	patternIndex := 0
	for offset, current := range candidate {
		if current != patternRunes[patternIndex] {
			continue
		}
		positions = append(positions, offset)
		patternIndex++
		if patternIndex == len(patternRunes) {
			return positions, true
		}
	}
	return nil, false
}
