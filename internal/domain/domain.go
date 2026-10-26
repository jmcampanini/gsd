package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
)

// ValidateTitle rejects a title that is blank or not valid UTF-8 with an
// invalid_argument error.
func ValidateTitle(title string) error {
	return ValidateRequiredText("title", title)
}

// ValidateRequiredText rejects a value that is blank or not valid UTF-8 with
// an invalid_argument error whose message names field.
func ValidateRequiredText(field, value string) error {
	if !utf8.ValidString(value) {
		return apperr.New(apperr.InvalidArgument, field+" must be valid UTF-8", nil)
	}
	if strings.TrimSpace(value) == "" {
		return apperr.New(apperr.InvalidArgument, field+" must not be blank", nil)
	}

	return nil
}

// ValidateNote rejects a note that is not valid UTF-8; an empty note is
// allowed.
func ValidateNote(note string) error {
	if !utf8.ValidString(note) {
		return apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}

	return nil
}

// NormalizeTagNames validates each name as a title and drops names that
// repeat an earlier one under SQLite NOCASE comparison, keeping the first
// spelling and the original order.
func NormalizeTagNames(names []string) ([]string, error) {
	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if err := ValidateTitle(name); err != nil {
			return nil, err
		}
		key := sqliteNoCaseKey(name)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, name)
	}

	return normalized, nil
}

func sqliteNoCaseKey(value string) string {
	var key strings.Builder
	key.Grow(len(value))
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character >= 'A' && character <= 'Z' {
			character += 'a' - 'A'
		}
		key.WriteByte(character)
	}
	return key.String()
}

// ValidateID rejects a non-positive ID with an invalid_argument error whose
// message names noun.
func ValidateID(noun string, id int64) error {
	if id <= 0 {
		return apperr.New(apperr.InvalidArgument, fmt.Sprintf("%s ID must be positive", noun), nil)
	}

	return nil
}

// ValidateOptionalID applies ValidateID when id is set and accepts nil.
func ValidateOptionalID(noun string, id *int64) error {
	if id == nil {
		return nil
	}

	return ValidateID(noun, *id)
}

// ParseID parses value as a positive decimal ID. Anything else, including
// signs, whitespace, and zero, is an invalid_argument error naming noun.
func ParseID(noun, value string) (int64, error) {
	if value == "" {
		return 0, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("%s ID must be a positive decimal", noun),
			nil,
		)
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, apperr.New(
				apperr.InvalidArgument,
				fmt.Sprintf("invalid %s ID %q", noun, value),
				nil,
			)
		}
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.New(
			apperr.InvalidArgument,
			fmt.Sprintf("invalid %s ID %q", noun, value),
			err,
		)
	}

	return id, nil
}

// FormatTimestamp renders value in UTC as the millisecond RFC 3339 form
// stored in timestamp columns.
func FormatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}

// NormalizeSliceResult turns a nil slice from a successful query into an
// empty one so JSON renders [] rather than null, and passes errors through.
func NormalizeSliceResult[T any](values []T, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	if values == nil {
		return []T{}, nil
	}

	return values, nil
}
