//go:build integration

package trip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
	"github.com/505Eclypsia/template/internal/model"
	repoTrip "github.com/505Eclypsia/template/internal/repository/trip"
)

func TestRepository_PositionsAndLastPositionAt(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	repo := repoTrip.NewTripRepo(pool, 3*time.Second)
	txManager := transaction.NewTxManager(pool)

	startedAt := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
	tripID := insertTrip(t, pool, "active", startedAt, nil)

	t1 := startedAt.Add(2 * time.Minute)
	tOlder := startedAt.Add(time.Minute)
	tNewer := startedAt.Add(4 * time.Minute)

	require.NoError(t, txManager.Do(ctx, func(txCtx context.Context) error {
		_, err := repo.CreateTripPosition(txCtx, model.TripPosition{
			TripID:     tripID,
			Latitude:   59.935,
			Longitude:  30.335,
			RecordedAt: t1,
		})
		if err != nil {
			return err
		}
		return repo.UpdateLastPositionAt(txCtx, tripID, t1)
	}))

	gotTrip, err := repo.GetTripByID(ctx, tripID)
	require.NoError(t, err)
	require.NotNil(t, gotTrip.LastPositionAt)
	assert.True(t, t1.Equal(*gotTrip.LastPositionAt))

	require.NoError(t, txManager.Do(ctx, func(txCtx context.Context) error {
		_, err := repo.CreateTripPosition(txCtx, model.TripPosition{
			TripID:     tripID,
			Latitude:   59.934,
			Longitude:  30.334,
			RecordedAt: tOlder,
		})
		if err != nil {
			return err
		}
		return repo.UpdateLastPositionAt(txCtx, tripID, tOlder)
	}))

	gotTrip, err = repo.GetTripByID(ctx, tripID)
	require.NoError(t, err)
	require.NotNil(t, gotTrip.LastPositionAt)
	assert.True(t, t1.Equal(*gotTrip.LastPositionAt), "старая координата не должна состарить поездку")

	errBoom := errors.New("simulated failure")
	err = txManager.Do(ctx, func(txCtx context.Context) error {
		if _, err := repo.CreateTripPosition(txCtx, model.TripPosition{
			TripID:     tripID,
			Latitude:   59.936,
			Longitude:  30.336,
			RecordedAt: tNewer,
		}); err != nil {
			return err
		}
		if err := repo.UpdateLastPositionAt(txCtx, tripID, tNewer); err != nil {
			return err
		}
		return errBoom
	})
	require.ErrorIs(t, err, errBoom)

	gotTrip, err = repo.GetTripByID(ctx, tripID)
	require.NoError(t, err)
	require.NotNil(t, gotTrip.LastPositionAt)
	assert.True(t, t1.Equal(*gotTrip.LastPositionAt), "last_position_at не должен измениться после отката")

	positions, err := repo.GetTripPositions(ctx, tripID)
	require.NoError(t, err)
	require.Len(t, positions, 2)
	assert.True(t, tOlder.Equal(positions[0].RecordedAt))
	assert.True(t, t1.Equal(positions[1].RecordedAt))
}
