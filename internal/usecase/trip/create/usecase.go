package create

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func (uc *UseCase) Create(ctx context.Context, trip *model.Trip, idempotencyKeyID *uuid.UUID) (*model.Trip, bool, error) {
	var tripHash string

	if idempotencyKeyID != nil {
		tIK, err := uc.repo.GetIdempotencyKeyByID(ctx, *idempotencyKeyID)
		if err != nil {
			return nil, false, err
		}

		tripHash = computeHash(*trip)

		if tIK != nil {
			if tripHash != tIK.RequestHash {
				return nil, false, domain.ErrIdempotencyConflict
			}

			existingTrip, err := uc.repo.GetTripByID(ctx, tIK.TripID)
			if err != nil {
				return nil, false, err
			}
			return existingTrip, false, nil
		}
	}

	if err := validate(trip); err != nil {
		return nil, false, err
	}

	tripID := uuid.New()
	trip.ID = tripID
	trip.StartedAt = time.Now()
	trip.Status = domain.StatusActive

	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := uc.repo.Create(txCtx, *trip); err != nil {
			return err
		}

		history := model.TripStatusHistory{
			TripID:     trip.ID,
			FromStatus: nil,
			ToStatus:   domain.StatusActive,
			ChangedAt:  trip.StartedAt,
		}

		if err := uc.repo.AddTripStatusHistory(txCtx, history); err != nil {
			return err
		}

		if idempotencyKeyID != nil {
			tIK := model.TripIdempotencyKey{
				ID:          *idempotencyKeyID,
				RequestHash: tripHash,
				TripID:      trip.ID,
			}
			return uc.repo.AddTripIdempotencyKey(txCtx, tIK)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	telemetry.Metrics.IncTripsCreated(ctx)
	return trip, true, nil
}

const errCoordsOutOfRange = "coordinates out of range"

func validate(trip *model.Trip) error {
	if trip.Price < 0 {
		return &domain.ValidationError{Field: "Price", Message: "price cannot be negative"}
	}

	switch {
	case trip.StartLatitude < -90 || trip.StartLatitude > 90:
		return &domain.ValidationError{Field: "StartLatitude", Message: errCoordsOutOfRange}
	case trip.StartLongitude < -180 || trip.StartLongitude > 180:
		return &domain.ValidationError{Field: "StartLongitude", Message: errCoordsOutOfRange}
	case trip.EndLatitude < -90 || trip.EndLatitude > 90:
		return &domain.ValidationError{Field: "EndLatitude", Message: errCoordsOutOfRange}
	case trip.EndLongitude < -180 || trip.EndLongitude > 180:
		return &domain.ValidationError{Field: "EndLongitude", Message: errCoordsOutOfRange}
	}
	return nil
}

func computeHash(t model.Trip) string {
	data := fmt.Sprintf("%s|%s|%.8f|%.8f|%.8f|%.8f|%d",
		t.UserID,
		t.DriverID,
		t.StartLatitude,
		t.StartLongitude,
		t.EndLatitude,
		t.EndLongitude,
		t.Price,
	)
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}
