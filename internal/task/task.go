package task

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
)

// The task status values, re-exported from domain.
const (
	StatusOpen      = domain.StatusOpen
	StatusDone      = domain.StatusDone
	StatusCancelled = domain.StatusCancelled
)

// ListStatus selects which tasks a listing returns by status.
type ListStatus string

// The list statuses: one stored status, or ListStatusAll for every status.
const (
	ListStatusOpen      ListStatus = StatusOpen
	ListStatusDone      ListStatus = StatusDone
	ListStatusCancelled ListStatus = StatusCancelled
	ListStatusAll       ListStatus = "all"
)

// DateSelector narrows a listing by the task's dates.
type DateSelector string

// The date selectors. DateSelectorNone applies no date filter; the others
// keep tasks with a due date, open tasks due before today, or tasks whose
// deferral is still in effect by date or stage.
const (
	DateSelectorNone     DateSelector = ""
	DateSelectorDue      DateSelector = "due"
	DateSelectorOverdue  DateSelector = "overdue"
	DateSelectorDeferred DateSelector = "deferred"
)

// ListOptions is the use-case filter for a task listing. Tag is a tag title;
// ProjectID and AreaID cannot both be set.
type ListOptions struct {
	Status    ListStatus
	Date      DateSelector
	ProjectID *int64
	AreaID    *int64
	Tag       *string
}

// ListFilter is the store-level filter after the service resolved the tag
// title to an ID.
type ListFilter struct {
	Status    ListStatus
	Date      DateSelector
	ProjectID *int64
	AreaID    *int64
	TagID     *int64
}

// Task is the task row re-exported from domain.
type Task = domain.Task

// Overdue reports whether an open task's due date has arrived: due on or
// before today, judged in the caller's local calendar date.
func Overdue(current Task, today string) bool {
	return current.Status == StatusOpen && current.DueOn != nil && *current.DueOn <= today
}

// ViewTask is a task joined with its project title and governing area for
// the inbox and available views.
type ViewTask struct {
	Task
	ProjectTitle       *string `json:"project_title"`
	GoverningAreaID    *int64  `json:"governing_area_id"`
	GoverningAreaTitle *string `json:"governing_area_title"`
}

// AddRequest is the use-case input for creating a task. Dates are in the
// DATE grammar, DeferStage is a stage title on the project's board, and
// Tags names existing tags to attach.
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

// AddFields is the store-level insert after the service canonicalized the
// dates and resolved the defer stage to an ID.
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

// DateChange sets a date, clears it, or leaves it; Set and Clear are
// mutually exclusive.
type DateChange struct {
	Set   *string
	Clear bool
}

// StageChange sets the defer stage by title, clears it, or leaves it.
type StageChange struct {
	Set   *string
	Clear bool
}

// IDChange is the store-level form of StageChange with the title resolved
// to a stage ID.
type IDChange struct {
	Set   *int64
	Clear bool
}

// ProjectChange moves a task into a project, out of its project, or leaves
// it.
type ProjectChange struct {
	Set   *int64
	Clear bool
}

// AreaChange moves a task into an area, out of its area, or leaves it.
type AreaChange struct {
	Set   *int64
	Clear bool
}

// EditRequest is the use-case input for editing a task; unset fields are
// left alone.
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

// EditFields is the store-level edit after the service canonicalized dates
// and resolved the defer stage.
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

// StageReference identifies a stage with its board and position, as the
// task store returns it.
type StageReference struct {
	ID       int64
	BoardID  int64
	Title    string
	Position int64
}

// Edition is the result of editing a task. ClearedDefers holds the task
// itself when leaving its project cleared its defer stage.
type Edition struct {
	Task          Task   `json:"task"`
	ClearedDefers []Task `json:"cleared_defers"`
}

// Promotion describes what completing a promoting task did to its project:
// the stage it moved to, or LastStage when it was already on the final
// stage.
type Promotion struct {
	Project    domain.Project
	StageTitle string
	LastStage  bool
}

// Completion is the result of marking a task done. PromotedProject is set
// only when the project moved to a new stage; Promotion also reports the
// last-stage case for human output.
type Completion struct {
	Task            Task            `json:"task"`
	PromotedProject *domain.Project `json:"promoted_project"`
	Promotion       *Promotion      `json:"-"`
}

// Tagging is the result of attaching or detaching tags: the refreshed task
// and the stored spellings of the tags named in the request.
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

// Store is the persistence boundary the service depends on; the read
// transaction gives filtered lists a consistent snapshot.
type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
	WithinReadTransaction(context.Context, func(Transaction) error) error
}

// Application is the task use-case surface the CLI and TUI consume.
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
