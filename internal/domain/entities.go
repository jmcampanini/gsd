// Entity row types live in domain so any package can reference them without
// import cycles; requests, projections, envelopes, and services stay in the
// entity packages, which re-export these rows under aliases.
package domain

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
