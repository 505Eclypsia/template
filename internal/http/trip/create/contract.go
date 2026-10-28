package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	Create(ctx context.Context, trip *model.Trip, idempotencyKeyID *uuid.UUID) (*model.Trip, bool, error)
}
