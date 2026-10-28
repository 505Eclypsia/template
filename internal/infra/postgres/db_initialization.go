package postgres

import (
	"context"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/internal/config"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}

	poolConfig.ConnConfig.Tracer = otelpgx.NewTracer()

	poolConfig.MaxConns = cfg.DatabaseMaxConns
	poolConfig.MinConns = cfg.DatabaseMinConns
	poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime

	connCtx, cancel := context.WithTimeout(ctx, cfg.DatabaseConnTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(connCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
