package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
)

func (t *TripRepo) Create(ctx context.Context, trip model.Trip) error {
	query, args, err := psql.Insert("trips").
		Columns(
			"id", "user_id", "driver_id", "start_latitude",
			"start_longitude", "end_latitude", "end_longitude",
			"price", "status", "started_at",
		).
		Values(
			trip.ID, trip.UserID, trip.DriverID, trip.StartLatitude,
			trip.StartLongitude, trip.EndLatitude, trip.EndLongitude,
			trip.Price, trip.Status, trip.StartedAt,
		).ToSql()
	if err != nil {
		return fmt.Errorf("create trip: %w", err)
	}

	_, err = t.exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDriverBusy
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
		return fmt.Errorf("query making: %w", err)
	}

	tag, err := t.exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("database update: %w", err)
	}

	if tag.RowsAffected() == 0 {
		query, args, err = psql.Select("status").From("trips").Where(sq.Eq{"id": id}).ToSql()
		if err != nil {
			return fmt.Errorf("query making: %w", err)
		}

		var status string
		err = t.queryRow(ctx, query, args...).Scan(&status)

		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTripNotFound
		}

		if err != nil {
			return fmt.Errorf("check trip status: %w", err)
		}

		if status == "completed" {
			return domain.ErrTripCompleted
		}
	}

	return nil
}

func (t *TripRepo) AddTripStatusHistory(ctx context.Context, tSH model.TripStatusHistory) error {
	query, args, err := psql.Insert("trip_status_history").
		Columns("trip_id", "from_status", "to_status", "reason", "changed_at").
		Values(tSH.TripID, tSH.FromStatus, tSH.ToStatus, tSH.Reason, tSH.ChangedAt).
		ToSql()
	if err != nil {
		return fmt.Errorf("making query: %w", err)
	}
	_, err = t.exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("database insert: %w", err)
	}
	return nil
}

func (t *TripRepo) AddTripIdempotencyKey(ctx context.Context, tIK model.TripIdempotencyKey) error {
	query, args, err := psql.Insert("trip_idempotency_keys").
		Columns("id", "request_hash", "trip_id").
		Values(tIK.ID, tIK.RequestHash, tIK.TripID).
		ToSql()
	if err != nil {
		return fmt.Errorf("query making: %w", err)
	}

	_, err = t.exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("database insert: %w", err)
	}

	return nil
}
