//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package finish

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type (
	tripRepository interface {
		Finish(ctx context.Context, id uuid.UUID) error
		AddTripStatusHistory(ctx context.Context, tSH model.TripStatusHistory) error
		GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)
