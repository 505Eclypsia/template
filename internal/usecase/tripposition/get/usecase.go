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

func (uc *UseCase) GetTripPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	if _, err := uc.repo.GetTripByID(ctx, tripID); err != nil {
		return nil, err
	}
	return uc.repo.GetTripPositions(ctx, tripID)
}
