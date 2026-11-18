//go:build integration

package trip_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	repoTrip "github.com/505Eclypsia/template/internal/repository/trip"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		//nolint:gosec // учебный локальный DSN для интеграционных тестов
		dsn = "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func insertTrip(
	t *testing.T, pool *pgxpool.Pool,
	status string, startedAt time.Time, lastPositionAt *time.Time,
) uuid.UUID {
	t.Helper()

	id := uuid.New()
	var finishedAt *time.Time
	if status == "completed" {
		f := startedAt.Add(time.Minute)
		finishedAt = &f
	}

	_, err := pool.Exec(context.Background(), `
		INSERT INTO trips (id, user_id, driver_id, start_latitude, start_longitude,
		                   end_latitude, end_longitude, price, status, started_at,
		                   finished_at, last_position_at)
		VALUES ($1, $2, $3, 59.93, 30.31, 59.92, 30.36, 100, $4, $5, $6, $7)`,
		id, uuid.New(), uuid.New(), status, startedAt, finishedAt, lastPositionAt)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM trips WHERE id = $1`, id)
	})
	return id
}

func TestRepository_ListStaleTrips(t *testing.T) {
	ctx := context.Background()
	pool := integrationPool(t)
	repo := repoTrip.NewTripRepo(pool, 3*time.Second)

	now := time.Now()
	old := now.Add(-10 * time.Minute)
	fresh := now.Add(-5 * time.Second)

	noPositions := insertTrip(t, pool, "active", old, nil)                       // координат не было, started_at давно
	stalePosition := insertTrip(t, pool, "active", old.Add(-time.Hour), &old)    // последняя координата давно
	freshPosition := insertTrip(t, pool, "active", old.Add(-time.Hour), &fresh)  // свежая координата
	justStarted := insertTrip(t, pool, "active", fresh, nil)                     // только что начата
	completed := insertTrip(t, pool, "completed", old.Add(-time.Hour), &old)     // завершена
	completedNoPos := insertTrip(t, pool, "completed", old.Add(-time.Hour), nil) // завершена, координат не было

	got, err := repo.ListStaleTrips(ctx, time.Minute)
	require.NoError(t, err)

	ids := make(map[uuid.UUID]int, len(got))
	for i, s := range got {
		ids[s.TripID] = i
	}

	require.Contains(t, ids, noPositions)
	require.Contains(t, ids, stalePosition)
	require.NotContains(t, ids, freshPosition)
	require.NotContains(t, ids, justStarted)
	require.NotContains(t, ids, completed)
	require.NotContains(t, ids, completedNoPos)

	for i := 1; i < len(got); i++ {
		require.False(t, got[i].LastActivityAt.Before(got[i-1].LastActivityAt),
			"выборка должна быть отсортирована от самых давно молчащих")
	}
}
