package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

const (
	KindTask    = "task"
	KindProject = "project"
	KindArea    = "area"
)

type Hit struct {
	Kind               string           `json:"kind"`
	Task               *task.Task       `json:"-"`
	Project            *project.Project `json:"-"`
	Area               *area.Area       `json:"-"`
	ProjectTitle       *string          `json:"-"`
	GoverningAreaTitle *string          `json:"-"`
}

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
		return marshalHit(struct {
			Kind string `json:"kind"`
			task.Task
		}{Kind: h.Kind, Task: *h.Task})
	case KindProject:
		if h.Project == nil {
			return nil, fmt.Errorf("search hit kind %q does not match its entity row", h.Kind)
		}
		return marshalHit(struct {
			Kind string `json:"kind"`
			project.Project
		}{Kind: h.Kind, Project: *h.Project})
	case KindArea:
		if h.Area == nil {
			return nil, fmt.Errorf("search hit kind %q does not match its entity row", h.Kind)
		}
		return marshalHit(struct {
			Kind string `json:"kind"`
			area.Area
		}{Kind: h.Kind, Area: *h.Area})
	default:
		return nil, fmt.Errorf("search hit has unknown kind %q", h.Kind)
	}
}

func marshalHit(value any) ([]byte, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n")), nil
}

type Store interface {
	Search(context.Context, string) ([]Hit, error)
}

type Application interface {
	Search(context.Context, string) ([]Hit, error)
}
