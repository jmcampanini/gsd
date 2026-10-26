package area

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

// ListSlice selects which areas a listing returns by archive state.
type ListSlice string

// The list slices. ListSliceActive is the default.
const (
	ListSliceActive   ListSlice = "active"
	ListSliceArchived ListSlice = "archived"
	ListSliceAll      ListSlice = "all"
)

// Area is the area row re-exported from domain.
type Area = domain.Area

// AddFields is the validated input for creating an area. Tags names existing
// tags to attach.
type AddFields struct {
	Title string
	Note  string
	Tags  []string
}

// EditFields holds the area fields an edit changes; a nil field is left
// alone.
type EditFields struct {
	Title *string
	Note  *string
}

// ListOptions filters an area listing.
type ListOptions struct {
	Slice ListSlice
}

// TaskDeletionScope selects which of an area's tasks a recursive delete
// removes in one pass.
type TaskDeletionScope string

// The deletion scopes: tasks that belong to the area's projects, and loose
// tasks filed directly in the area.
const (
	TaskDeletionScopeProject TaskDeletionScope = "project"
	TaskDeletionScopeLoose   TaskDeletionScope = "loose"
)

// Deletion is the result of deleting an area. The deleted slices are empty,
// never nil, when the delete was not recursive.
type Deletion struct {
	Area            Area              `json:"area"`
	DeletedProjects []project.Project `json:"deleted_projects"`
	DeletedTasks    []task.Task       `json:"deleted_tasks"`
}

// Tagging is the result of attaching or detaching tags: the refreshed area
// and the stored spellings of the tags named in the request.
type Tagging struct {
	Area      Area
	TagTitles []string
}

// ArchivedAreasError is the blocker error re-exported from domain.
type ArchivedAreasError = domain.ArchivedAreasError

// Transaction methods return areas, projects, and tasks with non-nil Tags slices.
type Transaction interface {
	Add(context.Context, AddFields, string) (Area, error)
	Find(context.Context, int64) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Edit(context.Context, int64, EditFields, string) (Area, error)
	Reorder(context.Context, int64, domain.Placement, string) (Area, error)
	Archive(context.Context, int64, string) (Area, error)
	Unarchive(context.Context, int64, string) (Area, error)
	Occupied(context.Context, int64) (bool, error)
	Delete(context.Context, int64) (Area, error)
	DeleteProjects(context.Context, int64) ([]project.Project, error)
	DeleteTasks(context.Context, int64, TaskDeletionScope) ([]task.Task, error)
	ResolveTags(context.Context, []string) ([]tag.Tag, error)
	AttachTags(context.Context, int64, []tag.Tag) error
	DetachTags(context.Context, int64, []tag.Tag) error
}

// Store is the persistence boundary the service depends on: the Transaction
// operations plus a way to run several of them in one write transaction.
type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
}

// Application is the area use-case surface the CLI and TUI consume.
type Application interface {
	Add(context.Context, AddFields) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Show(context.Context, int64) (Area, error)
	Edit(context.Context, int64, EditFields) (Area, error)
	Reorder(context.Context, int64, domain.Placement) (Area, error)
	Archive(context.Context, int64) (Area, error)
	Unarchive(context.Context, int64) (Area, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(context.Context, int64, bool) (Deletion, error)
}
