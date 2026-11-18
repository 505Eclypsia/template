package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type tripRepository interface {
	CreateTripPosition(ctx context.Context, tripPosition model.TripPosition) (*model.TripPosition, error)
	UpdateLastPositionAt(ctx context.Context, tripId uuid.UUID, lastPositionAt time.Time) error
	GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type positionCanceller interface {
	Cancel(tripID uuid.UUID)
}

type metrics interface {
	IncPositionsSaved(ctx context.Context)
}
