package search

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

func (s *Service) Search(ctx context.Context, expression string, related bool) ([]Hit, error) {
	if err := domain.ValidateRequiredText("expression", expression); err != nil {
		return nil, err
	}

	return domain.NormalizeSliceResult(s.store.Search(ctx, expression, related))
}
