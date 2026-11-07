package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/domain/trip"
)

type Trip struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	DriverID uuid.UUID

	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64

	Price  int64
	Status trip.Status

	StartedAt  time.Time
	FinishedAt *time.Time
}

type TripStatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *trip.Status
	ToStatus   trip.Status
	Reason     *string
	ChangedAt  time.Time
}

type TripIdempotencyKey struct {
	ID          uuid.UUID
	RequestHash string
	TripID      uuid.UUID
}
