package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
)

func ValidateTitle(title string) error {
	if !utf8.ValidString(title) {
		return apperr.New(apperr.InvalidArgument, "title must be valid UTF-8", nil)
	}
	if strings.TrimSpace(title) == "" {
		return apperr.New(apperr.InvalidArgument, "title must not be blank", nil)
	}

	return nil
}

func ValidateNote(note string) error {
	if !utf8.ValidString(note) {
		return apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}

	return nil
}

func ValidateID(noun string, id int64) error {
	if id <= 0 {
		return apperr.New(apperr.InvalidArgument, fmt.Sprintf("%s ID must be positive", noun), nil)
	}

	return nil
}

func ValidateOptionalID(noun string, id *int64) error {
	if id == nil {
		return nil
	}

	return ValidateID(noun, *id)
}

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

func FormatTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000Z")
}

func NormalizeSlice[T any](values []T) []T {
	if values == nil {
		return []T{}
	}

	return values
}

func NormalizeSliceResult[T any](values []T, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}

	return NormalizeSlice(values), nil
}
