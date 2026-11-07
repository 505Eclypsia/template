package db

import (
	"errors"
)

var (
	ErrDriverBusy             = errors.New("driver is busy")
	ErrTripNotFound           = errors.New("trip is not found")
	ErrTripAlreadyCompleted   = errors.New("trip already completed")
	ErrIdempotencyKeyNotFound = errors.New("idempotency key is not found")
)
