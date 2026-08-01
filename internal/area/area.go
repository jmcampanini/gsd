package area

import "context"

type ListSlice string

const (
	ListSliceActive   ListSlice = "active"
	ListSliceArchived ListSlice = "archived"
	ListSliceAll      ListSlice = "all"
)

type Area struct {
	ID         int64   `json:"id"`
	Title      string  `json:"title"`
	Note       string  `json:"note"`
	ArchivedAt *string `json:"archived_at"`
	Position   int64   `json:"position"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
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
	Slice ListSlice
}

type Store interface {
	Add(context.Context, AddFields, string) (Area, error)
	Find(context.Context, int64) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Edit(context.Context, int64, EditFields, string) (Area, error)
}

type Application interface {
	Add(context.Context, AddFields) (Area, error)
	List(context.Context, ListOptions) ([]Area, error)
	Show(context.Context, int64) (Area, error)
	Edit(context.Context, int64, EditFields) (Area, error)
}
