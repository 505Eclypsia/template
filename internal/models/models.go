package models

import (
	"time"

	"github.com/google/uuid"
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
	Status string

	StartedAt  time.Time
	FinishedAt *time.Time
}

type TripStatusHistory struct {
	ID         int64
	TripID     uuid.UUID
	FromStatus *string
	ToStatus   string
	Reason     *string
	ChangedAt  time.Time
}

type TripIdempotencyKey struct {
	ID          uuid.UUID
	RequestHash string
	TripID      uuid.UUID
}
