package board

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
	"github.com/jmcampanini/gsd/internal/task"
)

// Board is the board row re-exported from domain.
type Board = domain.Board

// Stage is the stage row re-exported from domain.
type Stage = domain.Stage

// AddFields is the input for creating a board with its initial stages in
// order; at least one stage is required.
type AddFields struct {
	Title  string
	Note   string
	Stages []string
}

// EditFields holds the board fields an edit changes; a nil field is left
// alone.
type EditFields struct {
	Title *string
	Note  *string
}

// Placement is a name-addressed reorder target: Reference is the sibling
// title for PlacementAfter and PlacementBefore and empty otherwise.
type Placement struct {
	Anchor    domain.PlacementAnchor
	Reference string
}

// Addition is the result of creating a board: the row and its stages in
// order.
type Addition struct {
	Board  Board
	Stages []Stage
}

// ListedBoard is a board with its stages in order, as listings return it.
type ListedBoard struct {
	Board
	Stages []Stage `json:"stages"`
}

// ProjectProgress counts a project's done tasks against its total.
type ProjectProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// ShownProject is a project on a board with its task progress.
type ShownProject struct {
	project.Project
	Progress ProjectProgress `json:"progress"`
}

// ShownStage is a stage with the projects placed on it in stage order.
type ShownStage struct {
	Stage
	Projects []ShownProject `json:"projects"`
}

// Show is the full board view: the row and every stage with its projects.
type Show struct {
	Board  Board        `json:"board"`
	Stages []ShownStage `json:"stages"`
}

// Deletion is the result of deleting a board: the row and the stages that
// went with it.
type Deletion struct {
	Board  Board   `json:"board"`
	Stages []Stage `json:"stages"`
}

// StageResult is a stage mutation result paired with the stage's board.
type StageResult struct {
	Board Board
	Stage Stage
}

// StageDeletion is the result of deleting a stage. ClearedDefers lists the
// tasks whose defer stage pointed at it and was cleared.
type StageDeletion struct {
	Stage         Stage       `json:"stage"`
	ClearedDefers []task.Task `json:"cleared_defers"`
	Board         Board       `json:"-"`
}

// StageRenameResult is the result of renaming a stage, keeping the title it
// had before.
type StageRenameResult struct {
	Board         Board
	Stage         Stage
	PreviousTitle string
}

// Occupancy counts the open and resolved projects on a board or stage.
type Occupancy struct {
	Open     int64
	Resolved int64
}

// Any reports whether any project occupies the board or stage.
func (o Occupancy) Any() bool {
	return o.Open > 0 || o.Resolved > 0
}

// Transaction is the set of board and stage store operations that can run on
// either the connection pool or one transaction. Name arguments match titles
// case-insensitively.
type Transaction interface {
	AddBoard(context.Context, AddFields, string) (Board, error)
	FindBoard(context.Context, string) (Board, error)
	FindBoardByID(context.Context, int64) (Board, error)
	ListBoards(context.Context) ([]Board, error)
	EditBoard(context.Context, int64, EditFields, string) (Board, error)
	ReorderBoard(context.Context, int64, domain.Placement, string) (Board, error)
	DeleteBoard(context.Context, int64) (Board, error)
	AddStage(context.Context, int64, string, string) (Stage, error)
	FindStage(context.Context, int64, string) (Stage, error)
	ListStages(context.Context, int64) ([]Stage, error)
	ListShownProjects(context.Context, int64) ([]ShownProject, error)
	BoardOccupancy(context.Context, int64) (Occupancy, error)
	StageOccupancy(context.Context, int64) (Occupancy, error)
	ClearTaskStageDefers(context.Context, int64, string) ([]task.Task, error)
	RenameStage(context.Context, int64, int64, string, string) (Stage, error)
	ReorderStage(context.Context, int64, int64, domain.Placement, string) (Stage, error)
	DeleteStage(context.Context, int64, int64) (Stage, error)
}

// Store is the persistence boundary the service depends on; the read
// transaction gives list and show a consistent snapshot.
type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
	WithinReadTransaction(context.Context, func(Transaction) error) error
}

// Application is the board use-case surface the CLI and TUI consume. Boards
// and stages are addressed by title except for ShowByID.
type Application interface {
	Add(context.Context, AddFields) (Addition, error)
	List(context.Context) ([]ListedBoard, error)
	Show(context.Context, string) (Show, error)
	ShowByID(context.Context, int64) (Show, error)
	Edit(context.Context, string, EditFields) (Board, error)
	Reorder(context.Context, string, Placement) (Board, error)
	Delete(context.Context, string) (Deletion, error)
	AddStage(context.Context, string, string, *Placement) (StageResult, error)
	RenameStage(context.Context, string, string, string) (StageRenameResult, error)
	ReorderStage(context.Context, string, string, Placement) (StageResult, error)
	DeleteStage(context.Context, string, string) (StageDeletion, error)
}
