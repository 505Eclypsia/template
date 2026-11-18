package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	GetTripPositions(ctx context.Context, tripId uuid.UUID) ([]model.TripPosition, error)
}
