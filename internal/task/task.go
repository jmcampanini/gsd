package task

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
)

type ListStatus string

const (
	ListStatusOpen      ListStatus = "open"
	ListStatusDone      ListStatus = "done"
	ListStatusCancelled ListStatus = "cancelled"
	ListStatusAll       ListStatus = "all"
)

type DateSelector string

const (
	DateSelectorNone     DateSelector = ""
	DateSelectorDue      DateSelector = "due"
	DateSelectorOverdue  DateSelector = "overdue"
	DateSelectorDeferred DateSelector = "deferred"
)

type ListOptions struct {
	Status    ListStatus
	Date      DateSelector
	ProjectID *int64
	AreaID    *int64
	Tag       *string
}

type ListFilter struct {
	Status    ListStatus
	Date      DateSelector
	ProjectID *int64
	AreaID    *int64
	TagID     *int64
}

type Task struct {
	ID          int64           `json:"id"`
	ProjectID   *int64          `json:"project_id"`
	AreaID      *int64          `json:"area_id"`
	Title       string          `json:"title"`
	Note        string          `json:"note"`
	DeferUntil  *string         `json:"defer_until"`
	DueOn       *string         `json:"due_on"`
	DoneAt      *string         `json:"done_at"`
	CancelledAt *string         `json:"cancelled_at"`
	Status      string          `json:"status"`
	Position    int64           `json:"position"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Tags        domain.TagNames `json:"tags"`
}

type ViewTask struct {
	Task
	ProjectTitle       *string `json:"project_title"`
	GoverningAreaID    *int64  `json:"governing_area_id"`
	GoverningAreaTitle *string `json:"governing_area_title"`
}

type AddFields struct {
	ProjectID  *int64
	AreaID     *int64
	Title      string
	Note       string
	DeferUntil *string
	DueOn      *string
	Tags       []string
}

type DateChange struct {
	Set   *string
	Clear bool
}

type ProjectChange struct {
	Set   *int64
	Clear bool
}

type AreaChange struct {
	Set   *int64
	Clear bool
}

type EditFields struct {
	Project    ProjectChange
	Area       AreaChange
	Title      *string
	Note       *string
	DeferUntil DateChange
	DueOn      DateChange
}

type Tagging struct {
	Task      Task
	TagTitles []string
}

// Transaction methods return tasks with non-nil Tags slices.
type Transaction interface {
	Add(ctx context.Context, fields AddFields, timestamp string) (Task, error)
	Inbox(ctx context.Context) ([]ViewTask, error)
	Available(ctx context.Context) ([]ViewTask, error)
	Find(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, filter ListFilter) ([]Task, error)
	ProjectExists(context.Context, int64) error
	AreaExists(context.Context, int64) error
	Edit(ctx context.Context, id int64, fields EditFields, timestamp string) (Task, error)
	Reorder(ctx context.Context, id int64, placement domain.Placement, timestamp string) (Task, error)
	Done(ctx context.Context, id int64, timestamp string) (Task, error)
	Cancel(ctx context.Context, id int64, timestamp string) (Task, error)
	Reopen(ctx context.Context, id int64, timestamp string) (Task, error)
	Delete(ctx context.Context, id int64) (Task, error)
	ResolveTags(context.Context, []string) ([]tag.Tag, error)
	AttachTags(context.Context, int64, []tag.Tag) error
	DetachTags(context.Context, int64, []tag.Tag) error
}

type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
	WithinReadTransaction(context.Context, func(Transaction) error) error
}

type Application interface {
	Add(ctx context.Context, fields AddFields) (Task, error)
	Inbox(ctx context.Context) ([]ViewTask, error)
	Available(ctx context.Context) ([]ViewTask, error)
	Show(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, options ListOptions) ([]Task, error)
	Edit(ctx context.Context, id int64, fields EditFields) (Task, error)
	Reorder(ctx context.Context, id int64, placement domain.Placement) (Task, error)
	Done(ctx context.Context, id int64) (Task, error)
	Cancel(ctx context.Context, id int64) (Task, error)
	Reopen(ctx context.Context, id int64) (Task, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(ctx context.Context, id int64) (Task, error)
}
