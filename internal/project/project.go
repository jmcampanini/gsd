package project

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

// The project status values, re-exported from domain.
const (
	StatusOpen      = domain.StatusOpen
	StatusDone      = domain.StatusDone
	StatusCancelled = domain.StatusCancelled
)

// ListStatus selects which projects a listing returns by status.
type ListStatus string

// The list statuses: one stored status, or ListStatusAll for every status.
const (
	ListStatusOpen      ListStatus = StatusOpen
	ListStatusDone      ListStatus = StatusDone
	ListStatusCancelled ListStatus = StatusCancelled
	ListStatusAll       ListStatus = "all"
)

// Exit names how a project is resolved.
type Exit string

// The project exits: ExitDone completes the project and ExitCancelled
// abandons it.
const (
	ExitDone      Exit = "done"
	ExitCancelled Exit = "cancelled"
)

// Project is the project row re-exported from domain.
type Project = domain.Project

// AddRequest is the use-case input for creating a project. Board names a
// board whose first stage the project starts on, and Tags names existing
// tags to attach.
type AddRequest struct {
	AreaID *int64
	Board  *string
	Title  string
	Note   string
	Tags   []string
}

// AddFields is the store-level insert after the service resolved the board
// to a stage ID.
type AddFields struct {
	AreaID  *int64
	StageID *int64
	Title   string
	Note    string
}

// AreaChange moves a project to an area, out of its area, or leaves it; Set
// and Clear are mutually exclusive.
type AreaChange struct {
	Set   *int64
	Clear bool
}

// BoardChange puts a project on a board by title, takes it off its board, or
// leaves it; Set and Clear are mutually exclusive.
type BoardChange struct {
	Set   *string
	Clear bool
}

// StageChange is the store-level form of BoardChange with the board
// resolved to a stage ID.
type StageChange struct {
	Set   *int64
	Clear bool
}

// EditRequest is the use-case input for editing a project; unset fields are
// left alone.
type EditRequest struct {
	Area  AreaChange
	Board BoardChange
	Title *string
	Note  *string
}

// EditFields is the store-level edit after the service resolved the board
// change to a stage change.
type EditFields struct {
	Area  AreaChange
	Stage StageChange
	Title *string
	Note  *string
}

// ListOptions filters a project listing by status and optionally by area.
type ListOptions struct {
	Status ListStatus
	AreaID *int64
}

// Location names the board and stage a project sits on.
type Location struct {
	BoardTitle string
	StageTitle string
}

// Detail is a project with its board location, nil when it is not on a
// board. JSON output carries the bare row.
type Detail struct {
	Project
	Location *Location `json:"-"`
}

// Edition is the result of editing a project. ClearedDefers lists the tasks
// whose defer stage was cleared because the project changed boards, and
// Location is the new board position for human output.
type Edition struct {
	Project       Project     `json:"project"`
	ClearedDefers []task.Task `json:"cleared_defers"`
	Location      *Location   `json:"-"`
}

// Movement is the result of moving a project to a stage on its board.
type Movement struct {
	Project    Project
	StageTitle string
}

// BoardReference identifies a board the project store looked up by title.
type BoardReference struct {
	ID    int64
	Title string
}

// StageReference identifies a stage with its board, as the project store
// returns it.
type StageReference struct {
	ID         int64
	BoardID    int64
	BoardTitle string
	Title      string
	Position   int64
}

// AreaReference carries what containment checks need from an area: its ID
// and whether it is archived.
type AreaReference struct {
	ID         int64
	ArchivedAt *string
}

// Resolution is the result of resolving a project with the open tasks that
// were cancelled alongside it.
type Resolution struct {
	Project        Project     `json:"project"`
	CancelledTasks []task.Task `json:"cancelled_tasks"`
}

// Deletion is the result of deleting a project. DeletedTasks is empty, never
// nil, when the delete was not recursive.
type Deletion struct {
	Project      Project     `json:"project"`
	DeletedTasks []task.Task `json:"deleted_tasks"`
}

// Tagging is the result of attaching or detaching tags: the refreshed
// project and the stored spellings of the tags named in the request.
type Tagging struct {
	Project   Project
	TagTitles []string
}

// ResolvedProjectsError is the blocker error re-exported from domain.
type ResolvedProjectsError = domain.ResolvedProjectsError

// ArchivedAreasError is the blocker error re-exported from domain.
type ArchivedAreasError = domain.ArchivedAreasError

// Transaction methods return projects and tasks with non-nil Tags slices.
type Transaction interface {
	Add(context.Context, AddFields, string) (Project, error)
	Find(context.Context, int64) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	AreaExists(context.Context, int64) error
	FindArea(context.Context, int64) (AreaReference, error)
	FindBoard(context.Context, string) (BoardReference, error)
	FindFirstStage(context.Context, int64) (*StageReference, error)
	FindStage(context.Context, int64, string) (StageReference, error)
	FindStageByID(context.Context, int64) (StageReference, error)
	Edit(context.Context, int64, EditFields, string) (Project, error)
	MoveStage(context.Context, int64, int64, domain.Placement, string) (Project, error)
	Reorder(context.Context, int64, domain.Placement, string) (Project, error)
	Resolve(context.Context, int64, Exit, string) (Project, error)
	CancelOpenTasks(context.Context, int64, string) ([]task.Task, error)
	ClearTaskStageDefers(context.Context, int64, string) ([]task.Task, error)
	Reopen(context.Context, int64, string) (Project, error)
	Delete(context.Context, int64) (Project, error)
	DeleteTasks(context.Context, int64) ([]task.Task, error)
	ResolveTags(context.Context, []string) ([]tag.Tag, error)
	AttachTags(context.Context, int64, []tag.Tag) error
	DetachTags(context.Context, int64, []tag.Tag) error
}

// Store is the persistence boundary the service depends on; the read
// transaction gives filtered lists and show a consistent snapshot.
type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
	WithinReadTransaction(context.Context, func(Transaction) error) error
}

// Application is the project use-case surface the CLI and TUI consume.
type Application interface {
	Add(context.Context, AddRequest) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	Show(context.Context, int64) (Detail, error)
	Edit(context.Context, int64, EditRequest) (Edition, error)
	Move(context.Context, int64, string, *domain.Placement) (Movement, error)
	Reorder(context.Context, int64, domain.Placement) (Project, error)
	Resolve(context.Context, int64, Exit) (Resolution, error)
	Reopen(context.Context, int64) (Project, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(context.Context, int64, bool) (Deletion, error)
}
