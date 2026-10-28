//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
	"github.com/505Eclypsia/template/internal/model"
	repoTrip "github.com/505Eclypsia/template/internal/repository/trip"
)

func TestRepository_TransactionRollback(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		//nolint:gosec // учебный локальный DSN для интеграционных тестов
		dsn = "postgres://tripgo:tripgo@localhost:22032/tripgo?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	repo := repoTrip.NewTripRepo(pool)
	txManager := transaction.NewTxManager(pool)

	tripID := uuid.New()
	testTrip := model.Trip{
		ID:             tripID,
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.93,
		StartLongitude: 30.31,
		EndLatitude:    59.92,
		EndLongitude:   30.36,
		Price:          1000,
		Status:         trip.StatusActive,
		StartedAt:      time.Now(),
	}

	_ = txManager.Do(ctx, func(txCtx context.Context) error {
		if err := repo.Create(txCtx, testTrip); err != nil {
			return err
		}
		return errors.New("simulated error to trigger rollback")
	})

	_, err = repo.GetTripByID(ctx, tripID)
	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("expected trip to be rolled back, but found it!")
	}
}
