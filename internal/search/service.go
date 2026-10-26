package search

import (
	"context"

	"github.com/jmcampanini/gsd/internal/domain"
)

// Service validates search requests and runs them against a Store.
type Service struct {
	store Store
}

// NewService returns a Service backed by store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Search runs a full-text expression, which must not be blank, and returns
// hits in rank order, never a nil slice on success.
func (s *Service) Search(ctx context.Context, expression string, related bool) ([]Hit, error) {
	if err := domain.ValidateRequiredText("expression", expression); err != nil {
		return nil, err
	}

	return domain.NormalizeSliceResult(s.store.Search(ctx, expression, related))
}
