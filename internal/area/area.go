package area

import (
	"context"
	"fmt"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

type ListSlice string

const (
	ListSliceActive   ListSlice = "active"
	ListSliceArchived ListSlice = "archived"
	ListSliceAll      ListSlice = "all"
)

type Area struct {
	ID         int64           `json:"id"`
	Title      string          `json:"title"`
	Note       string          `json:"note"`
	ArchivedAt *string         `json:"archived_at"`
	Position   int64           `json:"position"`
	CreatedAt  string          `json:"created_at"`
	UpdatedAt  string          `json:"updated_at"`
	Tags       domain.TagNames `json:"tags"`
}

type AddFields struct {
	Title string
	Note  string
	Tags  []string
}

type EditFields struct {
	Title *string
	Note  *string
}

type ListOptions struct {
	Slice ListSlice
}

type TaskDeletionScope string

const (
	TaskDeletionScopeProject TaskDeletionScope = "project"
	TaskDeletionScopeLoose   TaskDeletionScope = "loose"
)

type Deletion struct {
	Area            Area              `json:"area"`
	DeletedProjects []project.Project `json:"deleted_projects"`
	DeletedTasks    []task.Task       `json:"deleted_tasks"`
}

type Tagging struct {
	Area      Area
	TagTitles []string
}

type ArchivedAreasError struct {
	IDs []int64
}

func (e ArchivedAreasError) Error() string {
	return fmt.Sprintf("archived areas block this operation: %v", e.IDs)
}

// Transaction returns every area, project, and task with a non-nil Tags slice.
type Transaction interface {
	Add(context.Context, AddFields, string) (Area, error)
	Find(context.Context, int64) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Edit(context.Context, int64, EditFields, string) (Area, error)
	Archive(context.Context, int64, string) (Area, error)
	Unarchive(context.Context, int64, string) (Area, error)
	Delete(context.Context, int64) (Area, error)
	DeleteProjects(context.Context, int64) ([]project.Project, error)
	DeleteTasks(context.Context, int64, TaskDeletionScope) ([]task.Task, error)
	ResolveTags(context.Context, []string) ([]tag.Tag, error)
	AttachTags(context.Context, int64, []tag.Tag) error
	DetachTags(context.Context, int64, []tag.Tag) error
}

type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
}

type Application interface {
	Add(context.Context, AddFields) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Show(context.Context, int64) (Area, error)
	Edit(context.Context, int64, EditFields) (Area, error)
	Archive(context.Context, int64) (Area, error)
	Unarchive(context.Context, int64) (Area, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(context.Context, int64, bool) (Deletion, error)
}
