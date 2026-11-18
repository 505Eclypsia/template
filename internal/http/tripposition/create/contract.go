package create

import (
	"context"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	Create(ctx context.Context, tripPosition *model.TripPosition) (*model.TripPosition, error)
}
