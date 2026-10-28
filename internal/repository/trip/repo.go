package trip

import (
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type TripRepo struct {
	pool *pgxpool.Pool
}

func NewTripRepo(pool *pgxpool.Pool) *TripRepo {
	return &TripRepo{pool: pool}
}

func (t *TripRepo) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx, ok := transaction.FromContext(ctx); ok {
		return tx.Exec(ctx, sql, args...)
	}
	return t.pool.Exec(ctx, sql, args...)
}

func (t *TripRepo) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx, ok := transaction.FromContext(ctx); ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return t.pool.QueryRow(ctx, sql, args...)
}
