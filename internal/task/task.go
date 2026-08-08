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

type Task = domain.Task

type ViewTask struct {
	Task
	ProjectTitle       *string `json:"project_title"`
	GoverningAreaID    *int64  `json:"governing_area_id"`
	GoverningAreaTitle *string `json:"governing_area_title"`
}

type AddRequest struct {
	ProjectID  *int64
	AreaID     *int64
	Title      string
	Note       string
	DeferUntil *string
	DeferStage *string
	DueOn      *string
	Promotes   bool
	Tags       []string
}

type AddFields struct {
	ProjectID    *int64
	AreaID       *int64
	Title        string
	Note         string
	DeferUntil   *string
	DeferStageID *int64
	DueOn        *string
	Promotes     bool
	Tags         []string
}

type DateChange struct {
	Set   *string
	Clear bool
}

type StageChange struct {
	Set   *string
	Clear bool
}

type IDChange struct {
	Set   *int64
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

type EditRequest struct {
	Project    ProjectChange
	Area       AreaChange
	Title      *string
	Note       *string
	DeferUntil DateChange
	DeferStage StageChange
	DueOn      DateChange
	Promotes   *bool
}

type EditFields struct {
	Project      ProjectChange
	Area         AreaChange
	Title        *string
	Note         *string
	DeferUntil   DateChange
	DeferStageID IDChange
	DueOn        DateChange
	Promotes     *bool
}

type StageReference struct {
	ID       int64
	BoardID  int64
	Title    string
	Position int64
}

type Edition struct {
	Task          Task   `json:"task"`
	ClearedDefers []Task `json:"cleared_defers"`
}

type Promotion struct {
	Project    domain.Project
	StageTitle string
	LastStage  bool
}

type Completion struct {
	Task            Task            `json:"task"`
	PromotedProject *domain.Project `json:"promoted_project"`
	Promotion       *Promotion      `json:"-"`
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
	FindProject(context.Context, int64) (domain.Project, error)
	FindStage(ctx context.Context, boardID int64, title string) (StageReference, error)
	FindStageByID(context.Context, int64) (StageReference, error)
	StageExists(context.Context, string) (bool, error)
	FindNextStage(ctx context.Context, boardID, currentPosition int64) (*StageReference, error)
	MoveProjectStage(context.Context, int64, int64, string) (domain.Project, error)
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
	Add(ctx context.Context, fields AddRequest) (Task, error)
	Inbox(ctx context.Context) ([]ViewTask, error)
	Available(ctx context.Context) ([]ViewTask, error)
	Show(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, options ListOptions) ([]Task, error)
	Edit(ctx context.Context, id int64, fields EditRequest) (Edition, error)
	Reorder(ctx context.Context, id int64, placement domain.Placement) (Task, error)
	Done(ctx context.Context, id int64) (Completion, error)
	Cancel(ctx context.Context, id int64) (Task, error)
	Reopen(ctx context.Context, id int64) (Task, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(ctx context.Context, id int64) (Task, error)
}
