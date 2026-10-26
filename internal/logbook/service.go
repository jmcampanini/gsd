package logbook

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
)

// Service reads the logbook through a Store.
type Service struct {
	store Store
}

// NewService returns a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// List returns every entry, newest first, never a nil slice on success.
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	return domain.NormalizeSliceResult(s.store.List(ctx))
}
