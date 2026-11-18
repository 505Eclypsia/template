package get_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/tripposition/get"
)

type stubRepo struct {
	trip      *model.Trip
	tripErr   error
	positions []model.TripPosition
	posErr    error
}

func (r *stubRepo) GetTripByID(_ context.Context, _ uuid.UUID) (*model.Trip, error) {
	if r.tripErr != nil {
		return nil, r.tripErr
	}
	if r.trip == nil {
		return nil, domain.ErrTripNotFound
	}
	return r.trip, nil
}

func (r *stubRepo) GetTripPositions(_ context.Context, _ uuid.UUID) ([]model.TripPosition, error) {
	if r.posErr != nil {
		return nil, r.posErr
	}
	return r.positions, nil
}

func TestUseCase_GetTripPositions_Success(t *testing.T) {
	tripID := uuid.New()
	expected := []model.TripPosition{
		{ID: 1, TripID: tripID, Latitude: 59.93, Longitude: 30.31, RecordedAt: time.Now()},
	}
	uc := get.New(&stubRepo{
		trip:      &model.Trip{ID: tripID, Status: domain.StatusActive},
		positions: expected,
	})

	got, err := uc.GetTripPositions(context.Background(), tripID)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestUseCase_GetTripPositions_TripNotFound(t *testing.T) {
	uc := get.New(&stubRepo{tripErr: domain.ErrTripNotFound})

	_, err := uc.GetTripPositions(context.Background(), uuid.New())
	require.ErrorIs(t, err, domain.ErrTripNotFound)
}
