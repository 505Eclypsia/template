package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type UseCase struct {
	repo tripRepository
}

func New(repo tripRepository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) Get(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	trip, err := uc.repo.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return trip, nil
}
