package tag

import "context"

type Tag struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type ListedTag struct {
	Tag
	UsageCount int64 `json:"usage_count"`
}

func Titles(tags []Tag) []string {
	titles := make([]string, len(tags))
	for index := range tags {
		titles[index] = tags[index].Title
	}
	return titles
}

type Renaming struct {
	PreviousTitle string
	Tag           Tag
}

type Deletion struct {
	Tag      Tag   `json:"tag"`
	Detached int64 `json:"detached"`
}

type Store interface {
	Add(context.Context, string, string) (Tag, error)
	Find(context.Context, string) (Tag, error)
	List(context.Context) ([]ListedTag, error)
	Rename(context.Context, string, string, string) (Tag, error)
	CountUsage(context.Context, string) (int64, error)
	Delete(context.Context, string) (Tag, error)
	WithinTransaction(context.Context, func(Store) error) error
}

type Application interface {
	Add(context.Context, string) (Tag, error)
	List(context.Context) ([]ListedTag, error)
	Rename(context.Context, string, string) (Renaming, error)
	Delete(context.Context, string) (Deletion, error)
}
