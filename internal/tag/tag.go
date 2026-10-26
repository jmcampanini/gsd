package tag

import "context"

// Tag is one row of the tags table. Titles are unique under case-insensitive
// comparison.
type Tag struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListedTag is a tag with the number of tasks, projects, and areas it is
// attached to.
type ListedTag struct {
	Tag
	UsageCount int64 `json:"usage_count"`
}

// Titles returns the stored titles of tags in order.
func Titles(tags []Tag) []string {
	titles := make([]string, len(tags))
	for index := range tags {
		titles[index] = tags[index].Title
	}
	return titles
}

// Renaming is the result of renaming a tag: the row as renamed and the title
// it had before.
type Renaming struct {
	PreviousTitle string
	Tag           Tag
}

// Deletion is the result of deleting a tag. Detached counts the rows the tag
// was removed from.
type Deletion struct {
	Tag      Tag   `json:"tag"`
	Detached int64 `json:"detached"`
}

// Transaction is the set of tag store operations that can run on either the
// connection pool or one transaction. Name arguments match titles
// case-insensitively.
type Transaction interface {
	Add(context.Context, string, string) (Tag, error)
	Find(context.Context, string) (Tag, error)
	List(context.Context) ([]ListedTag, error)
	Rename(context.Context, string, string, string) (Tag, error)
	CountUsage(context.Context, string) (int64, error)
	Delete(context.Context, string) (Tag, error)
}

// Store is the persistence boundary the service depends on.
type Store interface {
	Transaction
	WithinTransaction(context.Context, func(Transaction) error) error
}

// Application is the tag use-case surface the CLI consumes.
type Application interface {
	Add(context.Context, string) (Tag, error)
	List(context.Context) ([]ListedTag, error)
	Rename(context.Context, string, string) (Renaming, error)
	Delete(context.Context, string) (Deletion, error)
}
