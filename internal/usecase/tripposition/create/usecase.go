package create

import (
	"context"
	"time"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/model"
)

const maxFutureSkew = time.Minute

type UseCase struct {
	repo      tripRepository
	txManager txManager
	canceller positionCanceller
	metrics   metrics
	now       func() time.Time
}

func New(
	repo tripRepository,
	txManager txManager,
	canceller positionCanceller,
	metrics metrics,
) *UseCase {
	return &UseCase{
		repo:      repo,
		txManager: txManager,
		canceller: canceller,
		metrics:   metrics,
		now:       time.Now,
	}
}

func (uc *UseCase) Create(ctx context.Context, tripPosition *model.TripPosition) (*model.TripPosition, error) {
	if tripPosition == nil {
		return nil, &domain.ValidationError{Field: "position", Message: "position must not be empty"}
	}
	if err := uc.validateInput(tripPosition); err != nil {
		return nil, err
	}

	var createdTripPosition *model.TripPosition
	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		trip, err := uc.repo.GetTripByID(txCtx, tripPosition.TripID)
		if err != nil {
			return err
		}
		if trip.Status == domain.StatusCompleted {
			return domain.ErrTripCompleted
		}

		created, err := uc.repo.CreateTripPosition(txCtx, *tripPosition)
		if err != nil {
			return err
		}
		createdTripPosition = created

		return uc.repo.UpdateLastPositionAt(txCtx, tripPosition.TripID, tripPosition.RecordedAt)
	})
	if err != nil {
		return nil, err
	}

	if uc.canceller != nil {
		uc.canceller.Cancel(tripPosition.TripID)
	}
	if uc.metrics != nil {
		uc.metrics.IncPositionsSaved(ctx)
	}

	return createdTripPosition, nil
}

func (uc *UseCase) validateInput(tripPosition *model.TripPosition) error {
	if err := domain.ValidateCoordinates("coordinates", tripPosition.Latitude, tripPosition.Longitude); err != nil {
		return err
	}
	if tripPosition.RecordedAt.IsZero() {
		return &domain.ValidationError{Field: "recorded_at", Message: "recorded_at must not be empty"}
	}
	if tripPosition.RecordedAt.After(uc.now().Add(maxFutureSkew)) {
		return &domain.ValidationError{Field: "recorded_at", Message: "recorded_at cannot be more than 1 minute in the future"}
	}
	return nil
}
