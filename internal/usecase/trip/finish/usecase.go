package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/infra/telemetry"
	"github.com/505Eclypsia/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
}

func New(repo tripRepository, txManager txManager) *UseCase {
	return &UseCase{repo: repo, txManager: txManager}
}

func (uc *UseCase) Finish(ctx context.Context, id uuid.UUID, changedAt time.Time) (*model.Trip, error) {
	trip, err := uc.repo.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if trip.Status != domain.StatusActive {
		return nil, domain.ErrTripCompleted
	}

	err = uc.txManager.Do(ctx, func(ctx context.Context) error {
		err := uc.repo.Finish(ctx, id)
		if err != nil {
			return err
		}

		reason := "trip finished"
		tripStatusHistory := model.TripStatusHistory{
			TripID:     id,
			FromStatus: &trip.Status,
			ToStatus:   domain.StatusCompleted,
			Reason:     &reason,
			ChangedAt:  changedAt,
		}

		err = uc.repo.AddTripStatusHistory(ctx, tripStatusHistory)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	trip.Status = domain.StatusCompleted
	trip.FinishedAt = &changedAt

	telemetry.Metrics.IncTripsCompleted(ctx)
	return trip, nil
}
