package trip

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type TripRepo struct {
	pool         *pgxpool.Pool
	queryTimeout time.Duration
}

func NewTripRepo(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepo {
	return &TripRepo{
		pool:         pool,
		queryTimeout: queryTimeout,
	}
}

func (t *TripRepo) LockIdempotencyKey(ctx context.Context, id uuid.UUID) error {
	_, err := t.exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", id.String())
	if err != nil {
		return fmt.Errorf("lock idempotency key: %w", err)
	}
	return nil
}

func (t *TripRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if t.queryTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, t.queryTimeout)
}

func (t *TripRepo) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	queryCtx, cancel := t.withTimeout(ctx)
	defer cancel()

	if tx, ok := transaction.FromContext(ctx); ok {
		return tx.Exec(queryCtx, sql, args...)
	}
	return t.pool.Exec(queryCtx, sql, args...)
}

type cancelRow struct {
	pgx.Row
	cancel context.CancelFunc
}

func (r cancelRow) Scan(dest ...any) error {
	defer r.cancel()
	return r.Row.Scan(dest...)
}

func (t *TripRepo) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	queryCtx, cancel := t.withTimeout(ctx)

	if tx, ok := transaction.FromContext(ctx); ok {
		return cancelRow{Row: tx.QueryRow(queryCtx, sql, args...), cancel: cancel}
	}
	return cancelRow{Row: t.pool.QueryRow(queryCtx, sql, args...), cancel: cancel}
}
