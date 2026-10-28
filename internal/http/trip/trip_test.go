package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/domain/trip"
)

func TestTrip_Finish(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		initial     trip.Status
		expectedErr error
	}{
		{
			name:        "success finish active trip",
			initial:     trip.StatusActive,
			expectedErr: nil,
		},
		{
			name:        "fail finish already completed trip",
			initial:     trip.StatusCompleted,
			expectedErr: trip.ErrTripCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &trip.Trip{
				ID:     uuid.New(),
				Status: tt.initial,
			}

			err := tr.Finish(now)
			if err != tt.expectedErr {
				t.Errorf("expected %v, got %v", tt.expectedErr, err)
			}

			if tt.expectedErr == nil && tr.Status != trip.StatusCompleted {
				t.Errorf("status must be completed")
			}
		})
	}
}
