package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
)

type UseCase struct {
	repo      tripRepository
	txManager txManager
	metrics   metrics
}

func New(repo tripRepository, txManager txManager, metrics metrics) *UseCase {
	return &UseCase{repo: repo, txManager: txManager, metrics: metrics}
}

func (uc *UseCase) Finish(ctx context.Context, id uuid.UUID, changedAt time.Time) (*model.Trip, error) {
	tripModel, err := uc.repo.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}

	domainTrip := domain.Trip{
		ID:         tripModel.ID,
		Status:     tripModel.Status,
		FinishedAt: tripModel.FinishedAt,
	}

	if err := domainTrip.Finish(changedAt); err != nil {
		return nil, err
	}

	fromStatus := tripModel.Status

	err = uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := uc.repo.Finish(txCtx, id, changedAt); err != nil {
			return err
		}

		reason := "trip finished"
		tripStatusHistory := model.TripStatusHistory{
			TripID:     id,
			FromStatus: &fromStatus,
			ToStatus:   "completed",
			Reason:     &reason,
			ChangedAt:  changedAt,
		}

		return uc.repo.AddTripStatusHistory(txCtx, tripStatusHistory)
	})
	if err != nil {
		return nil, err
	}

	tripModel.Status = domainTrip.Status
	tripModel.FinishedAt = domainTrip.FinishedAt

	uc.metrics.IncTripsCompleted(ctx)
	return tripModel, nil
}
