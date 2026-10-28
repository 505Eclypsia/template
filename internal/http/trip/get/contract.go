package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	Get(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
