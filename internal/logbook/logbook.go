package logbook

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
)

type Entry struct {
	Kind               string          `json:"kind"`
	ID                 int64           `json:"id"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	ResolvedAt         string          `json:"resolved_at"`
	ProjectTitle       *string         `json:"project_title"`
	GoverningAreaID    *int64          `json:"governing_area_id"`
	GoverningAreaTitle *string         `json:"governing_area_title"`
	Tags               domain.TagNames `json:"tags"`
}

// Store returns every entry with a non-nil Tags slice.
type Store interface {
	List(context.Context) ([]Entry, error)
}

type Application interface {
	List(context.Context) ([]Entry, error)
}
