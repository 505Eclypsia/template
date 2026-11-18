package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type tripRepository interface {
	GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	GetTripPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error)
}
