package search

import (
	"context"
	"fmt"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

// The entity kinds a Hit can carry.
const (
	KindTask    = "task"
	KindProject = "project"
	KindArea    = "area"
)

// Hit is one search result: a task, project, or area row tagged with its
// Kind. Exactly one entity pointer is populated and must match Kind;
// MarshalJSON enforces this. Constructors and accessors are deferred until
// the first new Hit consumer or producer (expected at the TUI milestones).
type Hit struct {
	Kind               string           `json:"kind"`
	Task               *task.Task       `json:"-"`
	Project            *project.Project `json:"-"`
	Area               *area.Area       `json:"-"`
	ProjectTitle       *string          `json:"-"`
	GoverningAreaTitle *string          `json:"-"`
}

// MarshalJSON flattens the populated entity row and prepends its kind, so a
// hit serializes as the row plus a "kind" field.
func (h Hit) MarshalJSON() ([]byte, error) {
	populated := 0
	if h.Task != nil {
		populated++
	}
	if h.Project != nil {
		populated++
	}
	if h.Area != nil {
		populated++
	}
	if populated != 1 {
		return nil, fmt.Errorf("search hit must contain exactly one entity row")
	}

	switch h.Kind {
	case KindTask:
		if h.Task == nil {
			return nil, fmt.Errorf("search hit kind %q does not match its entity row", h.Kind)
		}
		return domain.MarshalCompactJSON(struct {
			Kind string `json:"kind"`
			task.Task
		}{Kind: h.Kind, Task: *h.Task})
	case KindProject:
		if h.Project == nil {
			return nil, fmt.Errorf("search hit kind %q does not match its entity row", h.Kind)
		}
		return domain.MarshalCompactJSON(struct {
			Kind string `json:"kind"`
			project.Project
		}{Kind: h.Kind, Project: *h.Project})
	case KindArea:
		if h.Area == nil {
			return nil, fmt.Errorf("search hit kind %q does not match its entity row", h.Kind)
		}
		return domain.MarshalCompactJSON(struct {
			Kind string `json:"kind"`
			area.Area
		}{Kind: h.Kind, Area: *h.Area})
	default:
		return nil, fmt.Errorf("search hit has unknown kind %q", h.Kind)
	}
}

// Store runs a full-text query. The bool asks for related matches as well:
// rows whose project or area context matches, ranked after direct matches.
type Store interface {
	Search(context.Context, string, bool) ([]Hit, error)
}

// Application is the search use-case surface the CLI consumes.
type Application interface {
	Search(context.Context, string, bool) ([]Hit, error)
}
