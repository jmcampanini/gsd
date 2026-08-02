package logbook

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Entry, error) {
	return domain.NormalizeSliceResult(s.store.List(ctx))
}
