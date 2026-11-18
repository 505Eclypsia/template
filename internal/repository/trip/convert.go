package trip

import (
	"github.com/jackc/pgx/v5"

	"github.com/505Eclypsia/template/internal/model"
)

func scanTrip(row pgx.Row) (*model.Trip, error) {
	var t model.Trip
	err := row.Scan(
		&t.ID,
		&t.UserID,
		&t.DriverID,
		&t.StartLatitude,
		&t.StartLongitude,
		&t.EndLatitude,
		&t.EndLongitude,
		&t.Price,
		&t.Status,
		&t.StartedAt,
		&t.FinishedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanIdempotencyKey(row pgx.Row) (*model.TripIdempotencyKey, error) {
	var tIK model.TripIdempotencyKey
	err := row.Scan(
		&tIK.ID,
		&tIK.RequestHash,
		&tIK.TripID,
	)
	if err != nil {
		return nil, err
	}
	return &tIK, nil
}
