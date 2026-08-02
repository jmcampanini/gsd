package logbook

import "context"

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context) ([]Entry, error) {
	entries, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return []Entry{}, nil
	}
	for index := range entries {
		if entries[index].Tags == nil {
			entries[index].Tags = []string{}
		}
	}

	return entries, nil
}
