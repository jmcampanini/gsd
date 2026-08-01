package project

import "context"

type ListStatus string

const (
	ListStatusOpen      ListStatus = "open"
	ListStatusDone      ListStatus = "done"
	ListStatusCancelled ListStatus = "cancelled"
	ListStatusAll       ListStatus = "all"
)

type Project struct {
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

type AddFields struct {
	Title string
	Note  string
}

type EditFields struct {
	Title *string
	Note  *string
}

type ListOptions struct {
	Status ListStatus
}

type Store interface {
	Add(context.Context, AddFields, string) (Project, error)
	Find(context.Context, int64) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	Edit(context.Context, int64, EditFields, string) (Project, error)
}

type Application interface {
	Add(context.Context, AddFields) (Project, error)
	List(context.Context, ListOptions) ([]Project, error)
	Show(context.Context, int64) (Project, error)
	Edit(context.Context, int64, EditFields) (Project, error)
}
