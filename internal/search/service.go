package search

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/jmcampanini/gsd/internal/apperr"
	"github.com/jmcampanini/gsd/internal/domain"
)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Search(ctx context.Context, expression string) ([]Hit, error) {
	if !utf8.ValidString(expression) {
		return nil, apperr.New(apperr.InvalidArgument, "expression must be valid UTF-8", nil)
	}
	if strings.TrimSpace(expression) == "" {
		return nil, apperr.New(apperr.InvalidArgument, "expression must not be blank", nil)
	}

	return domain.NormalizeSliceResult(s.store.Search(ctx, expression))
}
