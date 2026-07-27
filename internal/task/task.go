package task

import "context"

type Task struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Note        string  `json:"note"`
	DoneAt      *string `json:"done_at"`
	CancelledAt *string `json:"cancelled_at"`
	Status      string  `json:"status"`
	Position    int64   `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type Repository interface {
	Add(context.Context, string, string, string) (Task, error)
	Inbox(context.Context) ([]Task, error)
	Find(context.Context, int64) (Task, error)
}

type Application interface {
	Add(context.Context, string, string) (Task, error)
	Inbox(context.Context) ([]Task, error)
	Show(context.Context, int64) (Task, error)
}
