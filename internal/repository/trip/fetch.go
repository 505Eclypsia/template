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
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("query making: %v", err)
	}

	tIK, err := scanIdempotencyKey(t.queryRow(ctx, query, args...))

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}

	return tIK, nil
}

func (t *TripRepo) GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	query, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at",
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
