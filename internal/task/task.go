package task

import "context"

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
	Status ListStatus
	Date   DateSelector
}

type Task struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Note        string  `json:"note"`
	DeferUntil  *string `json:"defer_until"`
	DueOn       *string `json:"due_on"`
	DoneAt      *string `json:"done_at"`
	CancelledAt *string `json:"cancelled_at"`
	Status      string  `json:"status"`
	Position    int64   `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type AddFields struct {
	Title      string
	Note       string
	DeferUntil *string
	DueOn      *string
}

type DateChange struct {
	Set   *string
	Clear bool
}

type EditFields struct {
	Title      *string
	Note       *string
	DeferUntil DateChange
	DueOn      DateChange
}

type Repository interface {
	Add(ctx context.Context, fields AddFields, timestamp string) (Task, error)
	Inbox(ctx context.Context) ([]Task, error)
	Available(ctx context.Context) ([]Task, error)
	Find(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, options ListOptions) ([]Task, error)
	Edit(ctx context.Context, id int64, fields EditFields, timestamp string) (Task, error)
	Done(ctx context.Context, id int64, timestamp string) (Task, error)
	Cancel(ctx context.Context, id int64, timestamp string) (Task, error)
	Reopen(ctx context.Context, id int64, timestamp string) (Task, error)
	Delete(ctx context.Context, id int64) (Task, error)
}

type Application interface {
	Add(ctx context.Context, fields AddFields) (Task, error)
	Inbox(ctx context.Context) ([]Task, error)
	Available(ctx context.Context) ([]Task, error)
	Show(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, options ListOptions) ([]Task, error)
	Edit(ctx context.Context, id int64, fields EditFields) (Task, error)
	Done(ctx context.Context, id int64) (Task, error)
	Cancel(ctx context.Context, id int64) (Task, error)
	Reopen(ctx context.Context, id int64) (Task, error)
	Delete(ctx context.Context, id int64) (Task, error)
}
