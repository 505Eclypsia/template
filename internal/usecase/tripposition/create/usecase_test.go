package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
	"github.com/505Eclypsia/template/internal/usecase/tripposition/create"
)

type stubTxKey struct{}

type stubRepo struct {
	trip         *model.Trip
	getErr       error
	createErr    error
	updateErr    error
	createInTx   bool
	updateInTx   bool
	createdCount int
}

func (r *stubRepo) GetTripByID(_ context.Context, _ uuid.UUID) (*model.Trip, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.trip == nil {
		return nil, domain.ErrTripNotFound
	}
	return r.trip, nil
}

func (r *stubRepo) CreateTripPosition(ctx context.Context, pos model.TripPosition) (*model.TripPosition, error) {
	r.createInTx = ctx.Value(stubTxKey{}) != nil
	if r.createErr != nil {
		return nil, r.createErr
	}
	r.createdCount++
	pos.ID = int64(r.createdCount)
	return &pos, nil
}

func (r *stubRepo) UpdateLastPositionAt(ctx context.Context, _ uuid.UUID, _ time.Time) error {
	r.updateInTx = ctx.Value(stubTxKey{}) != nil
	return r.updateErr
}

type stubTxManager struct{}

func (m *stubTxManager) Do(ctx context.Context, fn func(context.Context) error) error {
	txCtx := context.WithValue(ctx, stubTxKey{}, true)
	return fn(txCtx)
}

type stubCanceller struct {
	cancelled []uuid.UUID
}

func (c *stubCanceller) Cancel(tripID uuid.UUID) {
	c.cancelled = append(c.cancelled, tripID)
}

type stubMetrics struct {
	saved int
}

func (m *stubMetrics) IncPositionsSaved(_ context.Context) {
	m.saved++
}

func TestUseCase_Create_Success(t *testing.T) {
	tripID := uuid.New()
	repo := &stubRepo{
		trip: &model.Trip{ID: tripID, Status: domain.StatusActive},
	}
	canceller := &stubCanceller{}
	metrics := &stubMetrics{}
	uc := create.New(repo, &stubTxManager{}, canceller, metrics)

	now := time.Now().UTC()
	got, err := uc.Create(context.Background(), &model.TripPosition{
		TripID:     tripID,
		Latitude:   59.935,
		Longitude:  30.335,
		RecordedAt: now,
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.EqualValues(t, 1, got.ID)
	assert.True(t, repo.createInTx)
	assert.True(t, repo.updateInTx, "UpdateLastPositionAt must run inside the transaction context")
	assert.Equal(t, []uuid.UUID{tripID}, canceller.cancelled)
	assert.Equal(t, 1, metrics.saved)
}

func TestUseCase_Create_ValidationErrors(t *testing.T) {
	tripID := uuid.New()
	uc := create.New(&stubRepo{}, &stubTxManager{}, &stubCanceller{}, &stubMetrics{})

	tests := []struct {
		name string
		pos  *model.TripPosition
	}{
		{
			name: "nil position",
			pos:  nil,
		},
		{
			name: "latitude out of range",
			pos: &model.TripPosition{
				TripID:     tripID,
				Latitude:   91,
				Longitude:  30,
				RecordedAt: time.Now(),
			},
		},
		{
			name: "longitude out of range",
			pos: &model.TripPosition{
				TripID:     tripID,
				Latitude:   59,
				Longitude:  181,
				RecordedAt: time.Now(),
			},
		},
		{
			name: "zero recorded_at",
			pos: &model.TripPosition{
				TripID:    tripID,
				Latitude:  59,
				Longitude: 30,
			},
		},
		{
			name: "recorded_at too far in future",
			pos: &model.TripPosition{
				TripID:     tripID,
				Latitude:   59,
				Longitude:  30,
				RecordedAt: time.Now().Add(2 * time.Minute),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := uc.Create(context.Background(), tt.pos)
			var valErr *domain.ValidationError
			require.ErrorAs(t, err, &valErr)
		})
	}
}

func TestUseCase_Create_TripErrorsAndRollbackSafety(t *testing.T) {
	tripID := uuid.New()
	validPos := &model.TripPosition{
		TripID:     tripID,
		Latitude:   59.935,
		Longitude:  30.335,
		RecordedAt: time.Now(),
	}

	t.Run("trip not found", func(t *testing.T) {
		canceller := &stubCanceller{}
		metrics := &stubMetrics{}
		uc := create.New(&stubRepo{getErr: domain.ErrTripNotFound}, &stubTxManager{}, canceller, metrics)

		_, err := uc.Create(context.Background(), validPos)
		require.ErrorIs(t, err, domain.ErrTripNotFound)
		assert.Empty(t, canceller.cancelled)
		assert.Zero(t, metrics.saved)
	})

	t.Run("trip completed", func(t *testing.T) {
		canceller := &stubCanceller{}
		metrics := &stubMetrics{}
		repo := &stubRepo{trip: &model.Trip{ID: tripID, Status: domain.StatusCompleted}}
		uc := create.New(repo, &stubTxManager{}, canceller, metrics)

		_, err := uc.Create(context.Background(), validPos)
		require.ErrorIs(t, err, domain.ErrTripCompleted)
		assert.Empty(t, canceller.cancelled)
		assert.Zero(t, metrics.saved)
	})

	t.Run("update error skips cancel and metrics", func(t *testing.T) {
		dbErr := errors.New("db write error")
		canceller := &stubCanceller{}
		metrics := &stubMetrics{}
		repo := &stubRepo{
			trip:      &model.Trip{ID: tripID, Status: domain.StatusActive},
			updateErr: dbErr,
		}
		uc := create.New(repo, &stubTxManager{}, canceller, metrics)

		_, err := uc.Create(context.Background(), validPos)
		require.ErrorIs(t, err, dbErr)
		assert.Empty(t, canceller.cancelled)
		assert.Zero(t, metrics.saved)
	})
}
