package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	Finish(ctx context.Context, id uuid.UUID, changedAt time.Time) (*model.Trip, error)
}
