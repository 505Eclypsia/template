package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
	"github.com/505Eclypsia/template/internal/model"
)

const listStaleTripsSQL = `
SELECT id, driver_id, COALESCE(last_position_at, started_at) AS last_activity_at
FROM trips
WHERE status = 'active'
  AND COALESCE(last_position_at, started_at) < now() - make_interval(secs => $1)
ORDER BY COALESCE(last_position_at, started_at) ASC, id ASC`

func (t *TripRepo) ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	queryCtx, cancel := t.withTimeout(ctx)
	defer cancel()

	secs := staleAfter.Seconds()

	var (
		rows pgx.Rows
		err  error
	)
	if tx, ok := transaction.FromContext(ctx); ok {
		rows, err = tx.Query(queryCtx, listStaleTripsSQL, secs)
	} else {
		rows, err = t.pool.Query(queryCtx, listStaleTripsSQL, secs)
	}
	if err != nil {
		return nil, fmt.Errorf("list stale trips: %w", err)
	}
	defer rows.Close()

	var result []model.StaleTrip
	for rows.Next() {
		var st model.StaleTrip
		if err := rows.Scan(&st.TripID, &st.DriverID, &st.LastActivityAt); err != nil {
			return nil, fmt.Errorf("scan stale trip: %w", err)
		}
		result = append(result, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale trips: %w", err)
	}
	return result, nil
}
