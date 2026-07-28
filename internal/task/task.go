package task

import "context"

type ListStatus string

const (
	ListStatusOpen      ListStatus = "open"
	ListStatusDone      ListStatus = "done"
	ListStatusCancelled ListStatus = "cancelled"
	ListStatusAll       ListStatus = "all"
)

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

type EditFields struct {
	Title *string
	Note  *string
}

type Repository interface {
	Add(context.Context, string, string, string) (Task, error)
	Inbox(context.Context) ([]Task, error)
	Find(context.Context, int64) (Task, error)
	List(context.Context, ListStatus) ([]Task, error)
	Edit(context.Context, int64, EditFields, string) (Task, error)
	Done(context.Context, int64, string) (Task, error)
	Cancel(context.Context, int64, string) (Task, error)
	Reopen(context.Context, int64, string) (Task, error)
	Delete(context.Context, int64) (Task, error)
}

type Application interface {
	Add(context.Context, string, string) (Task, error)
	Inbox(context.Context) ([]Task, error)
	Show(context.Context, int64) (Task, error)
	List(context.Context, ListStatus) ([]Task, error)
	Edit(context.Context, int64, EditFields) (Task, error)
	Done(context.Context, int64) (Task, error)
	Cancel(context.Context, int64) (Task, error)
	Reopen(context.Context, int64) (Task, error)
	Delete(context.Context, int64) (Task, error)
}
