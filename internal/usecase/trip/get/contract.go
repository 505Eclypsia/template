//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type (
	tripRepository interface {
		GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}
)
