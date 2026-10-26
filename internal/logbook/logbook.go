package logbook

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
)

// The entry kinds a logbook row can have.
const (
	KindTask    = "task"
	KindProject = "project"
)

// Entry is one resolved task or project. ProjectTitle is set only for tasks
// in a project, and the governing area is the entry's own area or, for a
// task in a project, the project's area.
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

// Application is the logbook use-case surface the CLI and TUI consume.
type Application interface {
	List(context.Context) ([]Entry, error)
}
