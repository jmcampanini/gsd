package tag

import (
	"context"
	"time"

	"github.com/jmcampanini/gsd/internal/domain"
)

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Add(ctx context.Context, name string) (Tag, error) {
	if err := domain.ValidateTitle(name); err != nil {
		return Tag{}, err
	}

	return s.store.Add(ctx, name, domain.FormatTimestamp(s.now()))
}

func (s *Service) List(ctx context.Context) ([]ListedTag, error) {
	return domain.NormalizeSliceResult(s.store.List(ctx))
}

func (s *Service) Rename(ctx context.Context, oldName, newName string) (Renaming, error) {
	if err := domain.ValidateTitle(oldName); err != nil {
		return Renaming{}, err
	}
	if err := domain.ValidateTitle(newName); err != nil {
		return Renaming{}, err
	}

	var renaming Renaming
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		previous, err := store.Find(ctx, oldName)
		if err != nil {
			return err
		}

		renamed, err := store.Rename(ctx, oldName, newName, domain.FormatTimestamp(s.now()))
		if err != nil {
			return err
		}

		renaming = Renaming{PreviousTitle: previous.Title, Tag: renamed}
		return nil
	})
	if err != nil {
		return Renaming{}, err
	}

	return renaming, nil
}

func (s *Service) Delete(ctx context.Context, name string) (Deletion, error) {
	if err := domain.ValidateTitle(name); err != nil {
		return Deletion{}, err
	}

	var deletion Deletion
	err := s.store.WithinTransaction(ctx, func(store Store) error {
		if _, err := store.Find(ctx, name); err != nil {
			return err
		}

		detached, err := store.CountUsage(ctx, name)
		if err != nil {
			return err
		}

		deletedTag, err := store.Delete(ctx, name)
		if err != nil {
			return err
		}

		deletion = Deletion{Tag: deletedTag, Detached: detached}
		return nil
	})
	if err != nil {
		return Deletion{}, err
	}

	return deletion, nil
}
