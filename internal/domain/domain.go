package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
)

type Project struct {
	ID            int64    `json:"id"`
	AreaID        *int64   `json:"area_id"`
	Title         string   `json:"title"`
	Note          string   `json:"note"`
	DoneAt        *string  `json:"done_at"`
	CancelledAt   *string  `json:"cancelled_at"`
	Status        string   `json:"status"`
	Position      int64    `json:"position"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	StageID       *int64   `json:"stage_id"`
	StagePosition *int64   `json:"stage_position"`
	Tags          TagNames `json:"tags"`
}

func ValidateTitle(title string) error {
	return ValidateRequiredText("title", title)
}

func ValidateRequiredText(field, value string) error {
	if !utf8.ValidString(value) {
		return apperr.New(apperr.InvalidArgument, field+" must be valid UTF-8", nil)
	}
	if strings.TrimSpace(value) == "" {
		return apperr.New(apperr.InvalidArgument, field+" must not be blank", nil)
	}

	return nil
}

func ValidateNote(note string) error {
	if !utf8.ValidString(note) {
		return apperr.New(apperr.InvalidArgument, "note must be valid UTF-8", nil)
	}

	return nil
}

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

func NormalizeSliceResult[T any](values []T, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	if values == nil {
		return []T{}, nil
	}

	return values, nil
}
