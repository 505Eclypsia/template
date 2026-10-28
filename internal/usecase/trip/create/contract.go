//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type (
	tripRepository interface {
		Create(ctx context.Context, trip model.Trip) error
		AddTripStatusHistory(ctx context.Context, history model.TripStatusHistory) error
		GetIdempotencyKeyByID(ctx context.Context, id uuid.UUID) (*model.TripIdempotencyKey, error)
		AddTripIdempotencyKey(ctx context.Context, tIK model.TripIdempotencyKey) error
		GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}
)
