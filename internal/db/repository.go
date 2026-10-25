package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/models"
	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type TripRepo struct {
	pool *pgxpool.Pool
}

func NewTripRepo(pool *pgxpool.Pool) *TripRepo {
	return &TripRepo{pool: pool}
}

func (t *TripRepo) Create(ctx context.Context, trip models.Trip) error {
	query, args, err := psql.Insert("trips").
		Columns(
			"id", "user_id", "driver_id", "start_latitude",
			"start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at").
		Values(
			trip.ID, trip.UserID, trip.DriverID, trip.StartLatitude,
			trip.StartLongitude, trip.EndLatitude, trip.EndLongitude,
			trip.Price, trip.Status, trip.StartedAt).ToSql()

	if err != nil {
		return fmt.Errorf("create trip: %w", err)
	}

	_, err = t.exec(ctx, query, args...)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDriverBusy
		}
		return fmt.Errorf("create trip: %w", err)
	}

	return nil
}

func (t *TripRepo) Finish(ctx context.Context, id uuid.UUID) error {
	query, args, err := psql.Update("trips").
		Set("status", "completed").
		Set("finished_at", time.Now()).
		Where(sq.Eq{"id": id, "status": "active"}).
		ToSql()

	if err != nil {
		return fmt.Errorf("query making: %v", err)
	}

	tag, err := t.exec(ctx, query, args...)

	if err != nil {
		return fmt.Errorf("database update: %v", err)
	}

	if tag.RowsAffected() == 0 {
		query, args, err = psql.Select("status").From("trips").Where(sq.Eq{"id": id}).ToSql()
		if err != nil {
			return fmt.Errorf("query making: %v", err)
		}

		var status string
		err = t.queryRow(ctx, query, args...).Scan(&status)

		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTripNotFound
		}

		if err != nil {
			return fmt.Errorf("check trip status: %w", err)
		}

		if status == "completed" {
			return ErrTripAlreadyCompleted
		}
	}

	return nil
}

func (t *TripRepo) AddTripStatusHistory(ctx context.Context, tSH models.TripStatusHistory) error {
	query, args, err := psql.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(tSH.TripID, tSH.FromStatus, tSH.ToStatus, tSH.Reason, tSH.ChangedAt).
		ToSql()

	if err != nil {
		return fmt.Errorf("making query: %v", err)
	}
	_, err = t.exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("database insert: %v", err)
	}
	return nil
}

func (t *TripRepo) AddTripIdempotencyKey(ctx context.Context, tIK models.TripIdempotencyKey) error {
	query, args, err := psql.Insert("trip_idempotency_keys").
		Columns("id", "request_hash", "trip_id").
		Values(tIK.ID, tIK.RequestHash, tIK.TripID).
		ToSql()

	if err != nil {
		return fmt.Errorf("query making: %v", err)
	}

	_, err = t.exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("database insert: %v", err)
	}

	return nil
}

func (t *TripRepo) GetIdempotencyKeyByID(ctx context.Context, id uuid.UUID) (*models.TripIdempotencyKey, error) {
	query, args, err := psql.Select(
		"id", "request_hash", "trip_id").
		From("trip_idempotency_keys").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return nil, fmt.Errorf("query making: %v", err)
	}

	idempotencyKey := models.TripIdempotencyKey{}

	err = t.queryRow(ctx, query, args...).
		Scan(&idempotencyKey.ID, &idempotencyKey.RequestHash, &idempotencyKey.TripID)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIdempotencyKeyNotFound
	}

	return &idempotencyKey, nil
}

func (t *TripRepo) GetTripByID(ctx context.Context, id uuid.UUID) (*models.Trip, error) {
	query, args, err := psql.Select(
		"id", "user_id", "driver_id",
		"start_latitude", "start_longitude",
		"end_latitude", "end_longitude",
		"price", "status", "started_at", "finished_at").
		From("trips").Where(sq.Eq{"id": id}).ToSql()

	if err != nil {
		return nil, fmt.Errorf("query making: %v", err)
	}

	trip := models.Trip{}

	err = t.queryRow(ctx, query, args...).
		Scan(&trip.ID, &trip.UserID, &trip.DriverID, &trip.StartLatitude,
			&trip.StartLongitude, &trip.EndLatitude, &trip.EndLongitude,
			&trip.Price, &trip.Status, &trip.StartedAt, &trip.FinishedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrTripNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("database querying: %v", err)
	}
	return &trip, nil
}

func (t *TripRepo) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	return t.pool.Exec(ctx, sql, args...)
}

func (t *TripRepo) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return t.pool.QueryRow(ctx, sql, args...)
}
