// Package domain holds the entity row types and the validation and
// normalization rules shared by every entity package.
//
// Row types live here so any package can reference them without import
// cycles; requests, projections, envelopes, and services stay in the entity
// packages, which re-export these rows under aliases.
package domain

// Status values stored on task and project rows.
const (
	StatusOpen      = "open"
	StatusDone      = "done"
	StatusCancelled = "cancelled"
)

// Area is one row of the areas table. ArchivedAt is nil while the area is
// active.
type Area struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Note       string   `json:"note"`
	ArchivedAt *string  `json:"archived_at"`
	Position   int64    `json:"position"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	Tags       TagNames `json:"tags"`
}

// Board is one row of the boards table.
type Board struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Note      string `json:"note"`
	Position  int64  `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Stage is one row of the stages table; Position orders stages within
// their board.
type Stage struct {
	ID        int64  `json:"id"`
	BoardID   int64  `json:"board_id"`
	Title     string `json:"title"`
	Position  int64  `json:"position"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Project is one row of the projects table joined with its tag names.
// StageID and StagePosition are nil when the project is not on a board.
type Project struct {
	ID            int64    `json:"id"`
	AreaID        *int64   `json:"area_id"`
	Title         string   `json:"title"`
	Note          string   `json:"note"`
	DoneAt        *string  `json:"done_at"`
	CancelledAt   *string  `json:"cancelled_at"`
	Status        string   `json:"status"`
	Position      int64    `json:"position"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	StageID       *int64   `json:"stage_id"`
	StagePosition *int64   `json:"stage_position"`
	Tags          TagNames `json:"tags"`
}

// Task is one row of the tasks table joined with its tag names.
// DeferStageTitle is populated only by queries that join the defer stage
// and is omitted from JSON.
type Task struct {
	ID              int64    `json:"id"`
	ProjectID       *int64   `json:"project_id"`
	AreaID          *int64   `json:"area_id"`
	Title           string   `json:"title"`
	Note            string   `json:"note"`
	DeferUntil      *string  `json:"defer_until"`
	DueOn           *string  `json:"due_on"`
	DoneAt          *string  `json:"done_at"`
	CancelledAt     *string  `json:"cancelled_at"`
	Status          string   `json:"status"`
	Position        int64    `json:"position"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	DeferStageID    *int64   `json:"defer_stage_id"`
	Promotes        bool     `json:"promotes"`
	Tags            TagNames `json:"tags"`
	DeferStageTitle *string  `json:"-"`
}
