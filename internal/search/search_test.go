package search

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jmcampanini/gsd/internal/area"
	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

func TestHitMarshalJSONFlattensCanonicalEntityAndExcludesContext(t *testing.T) {
	t.Parallel()

	projectTitle := "Bathroom plumbing"
	areaTitle := "Home"
	taskRow := task.Task{ID: 1, Title: "Fix sink", Status: "open", Tags: domain.TagNames{"reno"}}
	projectRow := project.Project{ID: 2, Title: "Bathroom plumbing", Status: "open", Tags: domain.TagNames{}}
	areaRow := area.Area{ID: 3, Title: "Home", Tags: domain.TagNames{"house"}}

	for _, test := range []struct {
		name   string
		hit    Hit
		entity any
	}{
		{
			name: "task",
			hit: Hit{
				Kind:               KindTask,
				Task:               &taskRow,
				ProjectTitle:       &projectTitle,
				GoverningAreaTitle: &areaTitle,
			},
			entity: taskRow,
		},
		{
			name: "project",
			hit: Hit{
				Kind:               KindProject,
				Project:            &projectRow,
				GoverningAreaTitle: &areaTitle,
			},
			entity: projectRow,
		},
		{name: "area", hit: Hit{Kind: KindArea, Area: &areaRow}, entity: areaRow},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gotBytes, err := json.Marshal(test.hit)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(gotBytes, &got); err != nil {
				t.Fatalf("Unmarshal(hit) error = %v", err)
			}
			entityBytes, err := json.Marshal(test.entity)
			if err != nil {
				t.Fatalf("Marshal(entity) error = %v", err)
			}
			var want map[string]any
			if err := json.Unmarshal(entityBytes, &want); err != nil {
				t.Fatalf("Unmarshal(entity) error = %v", err)
			}
			want["kind"] = test.hit.Kind

			if !reflect.DeepEqual(got, want) {
				t.Errorf("Marshal() object = %#v, want flattened canonical row %#v", got, want)
			}
			for _, excluded := range []string{
				"task",
				"project",
				"area",
				"project_title",
				"governing_area_title",
			} {
				if _, exists := got[excluded]; exists {
					t.Errorf("Marshal() object contains excluded field %q", excluded)
				}
			}
		})
	}
}

func TestHitMarshalJSONRejectsInvalidEntityInvariant(t *testing.T) {
	t.Parallel()

	taskRow := task.Task{ID: 1}
	projectRow := project.Project{ID: 2}

	for _, test := range []struct {
		name string
		hit  Hit
	}{
		{name: "no row", hit: Hit{Kind: KindTask}},
		{
			name: "multiple rows",
			hit:  Hit{Kind: KindTask, Task: &taskRow, Project: &projectRow},
		},
		{name: "kind does not match row", hit: Hit{Kind: KindProject, Task: &taskRow}},
		{name: "unknown kind", hit: Hit{Kind: "note", Task: &taskRow}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := json.Marshal(test.hit); err == nil {
				t.Error("Marshal() error = nil, want invariant error")
			}
		})
	}
}
