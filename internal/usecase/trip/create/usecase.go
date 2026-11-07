package create

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func (uc *UseCase) Create(ctx context.Context, trip *model.Trip, idempotencyKeyID *uuid.UUID) (*model.Trip, bool, error) {
	if err := validate(trip); err != nil {
		return nil, false, err
	}

	var tripHash string
	if idempotencyKeyID != nil {
		tripHash = computeHash(*trip)
	}

	trip.ID = uuid.New()
	trip.StartedAt = time.Now()
	trip.Status = domain.StatusActive

	var existingTrip *model.Trip

	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if idempotencyKeyID != nil {
			if err := uc.repo.LockIdempotencyKey(txCtx, *idempotencyKeyID); err != nil {
				return err
			}

			tIK, err := uc.repo.GetIdempotencyKeyByID(txCtx, *idempotencyKeyID)
			if err != nil {
				return err
			}

			if tIK != nil {
				if tIK.RequestHash != tripHash {
					return domain.ErrIdempotencyConflict
				}

				found, err := uc.repo.GetTripByID(txCtx, tIK.TripID)
				if err != nil {
					return err
				}
				existingTrip = found
				return nil
			}
		}

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
			return uc.repo.AddTripIdempotencyKey(txCtx, model.TripIdempotencyKey{
				ID:          *idempotencyKeyID,
				RequestHash: tripHash,
				TripID:      trip.ID,
			})
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if existingTrip != nil {
		return existingTrip, false, nil
	}

	uc.metrics.IncTripsCreated(ctx)
	return trip, true, nil
}

const errCoordsOutOfRange = "coordinates out of range"

func validate(trip *model.Trip) error {
	if trip.UserID == uuid.Nil {
		return &domain.ValidationError{Field: "UserID", Message: "user id must not be empty"}
	}
	if trip.DriverID == uuid.Nil {
		return &domain.ValidationError{Field: "DriverID", Message: "driver id must not be empty"}
	}
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
