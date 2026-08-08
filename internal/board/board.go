package board

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
	"github.com/jmcampanini/gsd/internal/project"
)

type Board struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Note      string `json:"note"`
	Position  int64  `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type Stage struct {
	ID        int64  `json:"id"`
	BoardID   int64  `json:"board_id"`
	Title     string `json:"title"`
	Position  int64  `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type AddFields struct {
	Title  string
	Note   string
	Stages []string
}

type EditFields struct {
	Title *string
	Note  *string
}

type Placement struct {
	Anchor    domain.PlacementAnchor
	Reference string
}

type Addition struct {
	Board  Board
	Stages []Stage
}

type ListedBoard struct {
	Board
	Stages []Stage `json:"stages"`
}

type ProjectProgress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

type ShownProject struct {
	project.Project
	Progress ProjectProgress `json:"progress"`
}

type ShownStage struct {
	Stage
	Projects []ShownProject `json:"projects"`
}

type Show struct {
	Board  Board        `json:"board"`
	Stages []ShownStage `json:"stages"`
}

type Deletion struct {
	Board  Board   `json:"board"`
	Stages []Stage `json:"stages"`
}

type StageResult struct {
	Board Board
	Stage Stage
}

type StageRenameResult struct {
	Board         Board
	Stage         Stage
	PreviousTitle string
}

type Transaction interface {
	AddBoard(context.Context, AddFields, string) (Board, error)
	FindBoard(context.Context, string) (Board, error)
	ListBoards(context.Context) ([]Board, error)
	EditBoard(context.Context, int64, EditFields, string) (Board, error)
	ReorderBoard(context.Context, int64, domain.Placement, string) (Board, error)
	DeleteBoard(context.Context, int64) (Board, error)
	AddStage(context.Context, int64, string, string) (Stage, error)
	FindStage(context.Context, int64, string) (Stage, error)
	ListStages(context.Context, int64) ([]Stage, error)
	ListShownProjects(context.Context, int64) ([]ShownProject, error)
	BoardOccupied(context.Context, int64) (bool, error)
	StageOccupied(context.Context, int64) (bool, error)
	RenameStage(context.Context, int64, int64, string, string) (Stage, error)
	ReorderStage(context.Context, int64, int64, domain.Placement, string) (Stage, error)
	DeleteStage(context.Context, int64, int64) (Stage, error)
}

type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
	WithinReadTransaction(context.Context, func(Transaction) error) error
}

type Application interface {
	Add(context.Context, AddFields) (Addition, error)
	List(context.Context) ([]ListedBoard, error)
	Show(context.Context, string) (Show, error)
	Edit(context.Context, string, EditFields) (Board, error)
	Reorder(context.Context, string, Placement) (Board, error)
	Delete(context.Context, string) (Deletion, error)
	AddStage(context.Context, string, string, Placement) (StageResult, error)
	RenameStage(context.Context, string, string, string) (StageRenameResult, error)
	ReorderStage(context.Context, string, string, Placement) (StageResult, error)
	DeleteStage(context.Context, string, string) (StageResult, error)
}
