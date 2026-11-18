package trip

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
)

func (t *TripRepo) GetIdempotencyKeyByID(ctx context.Context, id uuid.UUID) (*model.TripIdempotencyKey, error) {
	query, args, err := psql.Select(
		"id", "request_hash", "trip_id",
	).
		From("trip_idempotency_keys").
		Where(sq.Eq{"id": id}).
		Where("created_at >= now() - interval '24 hours'").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("query making: %w", err)
	}

	tIK, err := scanIdempotencyKey(t.queryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("database querying: %w", err)
	}

	return tIK, nil
}

func (t *TripRepo) GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	query, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at", "last_position_at",
	).
		From("trips").Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("query making: %w", err)
	}

	trip, err := scanTrip(t.queryRow(ctx, query, args...))

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrTripNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("database querying: %w", err)
	}
	return trip, nil
}

func (t *TripRepo) GetTripPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	query, args, err := psql.Select(
		"id",
		"trip_id",
		"latitude",
		"longitude",
		"recorded_at",
	).
		From("trip_positions").
		Where(sq.Eq{"trip_id": tripID}).
		OrderBy("recorded_at ASC", "id ASC").
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("query making: %w", err)
	}

	rows, err := t.query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query: %w", err)
	}
	defer rows.Close()

	positions := make([]model.TripPosition, 0)
	for rows.Next() {
		var p model.TripPosition
		if err := rows.Scan(
			&p.ID,
			&p.TripID,
			&p.Latitude,
			&p.Longitude,
			&p.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("row scan: %w", err)
		}
		positions = append(positions, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return positions, nil
}
