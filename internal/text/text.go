// Package text holds presentation-safe text transforms shared by the
// CLI and TUI surfaces.
package text

import (
	"strconv"
	"strings"
	"unicode"
)

// Human escapes control characters into their visible quoted forms so
// stored values cannot smuggle terminal control sequences into output.
func Human(value string, preserveLineFeeds bool) string {
	var visible strings.Builder
	visible.Grow(len(value))
	for _, character := range value {
		if character == '\n' && preserveLineFeeds {
			visible.WriteRune(character)
			continue
		}
		if unicode.IsControl(character) {
			quoted := strconv.QuoteRune(character)
			visible.WriteString(quoted[1 : len(quoted)-1])
			continue
		}
		visible.WriteRune(character)
	}
	return visible.String()
}
