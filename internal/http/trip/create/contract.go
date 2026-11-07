//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package create

import (
	"context"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/model"
)

type useCase interface {
	Create(ctx context.Context, trip *model.Trip, idempotencyKeyID *uuid.UUID) (*model.Trip, bool, error)
}
