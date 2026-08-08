package project

import (
	"context"
	"fmt"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/tag"
	"github.com/jmcampanini/gsd/internal/task"
)

type ListStatus string

const (
	ListStatusOpen      ListStatus = "open"
	ListStatusDone      ListStatus = "done"
	ListStatusCancelled ListStatus = "cancelled"
	ListStatusAll       ListStatus = "all"
)

type Exit string

const (
	ExitDone      Exit = "done"
	ExitCancelled Exit = "cancelled"
)

type Project struct {
	ID            int64           `json:"id"`
	AreaID        *int64          `json:"area_id"`
	Title         string          `json:"title"`
	Note          string          `json:"note"`
	DoneAt        *string         `json:"done_at"`
	CancelledAt   *string         `json:"cancelled_at"`
	Status        string          `json:"status"`
	Position      int64           `json:"position"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
	StageID       *int64          `json:"stage_id"`
	StagePosition *int64          `json:"stage_position"`
	Tags          domain.TagNames `json:"tags"`
}

type AddFields struct {
	AreaID *int64
	Board  *string
	Title  string
	Note   string
	Tags   []string
}

type CreateFields struct {
	AreaID  *int64
	StageID *int64
	Title   string
	Note    string
}

type AreaChange struct {
	Set   *int64
	Clear bool
}

type BoardChange struct {
	Set   *string
	Clear bool
}

type StageChange struct {
	Set   *int64
	Clear bool
}

type EditFields struct {
	Area  AreaChange
	Board BoardChange
	Title *string
	Note  *string
}

type UpdateFields struct {
	Area  AreaChange
	Stage StageChange
	Title *string
	Note  *string
}

type ListOptions struct {
	Status ListStatus
	AreaID *int64
}

type Location struct {
	BoardTitle string
	StageTitle string
}

type Detail struct {
	Project
	Location *Location `json:"-"`
}

type Edition struct {
	Project       Project     `json:"project"`
	ClearedDefers []task.Task `json:"cleared_defers"`
	Location      *Location   `json:"-"`
}

type Movement struct {
	Project    Project
	StageTitle string
}

type BoardReference struct {
	ID    int64
	Title string
}

type StageReference struct {
	ID         int64
	BoardID    int64
	BoardTitle string
	Title      string
	Position   int64
}

type AreaReference struct {
	ID         int64
	ArchivedAt *string
}

type Resolution struct {
	Project        Project     `json:"project"`
	CancelledTasks []task.Task `json:"cancelled_tasks"`
}

type Deletion struct {
	Project      Project     `json:"project"`
	DeletedTasks []task.Task `json:"deleted_tasks"`
}

type Tagging struct {
	Project   Project
	TagTitles []string
}

type ResolvedProjectsError struct {
	IDs []int64
}

func (e ResolvedProjectsError) Error() string {
	return fmt.Sprintf("resolved projects block this operation: %v", e.IDs)
}

type ArchivedAreasError struct {
	IDs []int64
}

func (e ArchivedAreasError) Error() string {
	return fmt.Sprintf("archived areas block this operation: %v", e.IDs)
}

// Transaction methods return projects and tasks with non-nil Tags slices.
type Transaction interface {
	Add(context.Context, CreateFields, string) (Project, error)
	Find(context.Context, int64) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	AreaExists(context.Context, int64) error
	FindArea(context.Context, int64) (AreaReference, error)
	FindBoard(context.Context, string) (BoardReference, error)
	FindFirstStage(context.Context, int64) (*StageReference, error)
	FindStage(context.Context, int64, string) (StageReference, error)
	FindStageByID(context.Context, int64) (StageReference, error)
	Edit(context.Context, int64, UpdateFields, string) (Project, error)
	MoveStage(context.Context, int64, int64, domain.Placement, string) (Project, error)
	Reorder(context.Context, int64, domain.Placement, string) (Project, error)
	Resolve(context.Context, int64, Exit, string) (Project, error)
	CancelOpenTasks(context.Context, int64, string) ([]task.Task, error)
	Reopen(context.Context, int64, string) (Project, error)
	Delete(context.Context, int64) (Project, error)
	DeleteTasks(context.Context, int64) ([]task.Task, error)
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
	Add(context.Context, AddFields) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	Show(context.Context, int64) (Detail, error)
	Edit(context.Context, int64, EditFields) (Edition, error)
	Move(context.Context, int64, string, *domain.Placement) (Movement, error)
	Reorder(context.Context, int64, domain.Placement) (Project, error)
	Resolve(context.Context, int64, Exit) (Resolution, error)
	Reopen(context.Context, int64) (Project, error)
	Tag(context.Context, int64, []string) (Tagging, error)
	Untag(context.Context, int64, []string) (Tagging, error)
	Delete(context.Context, int64, bool) (Deletion, error)
}
