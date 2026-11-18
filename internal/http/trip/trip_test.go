package trip_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				assert.Equal(t, tt.initial, tr.Status, "статус не должен меняться при ошибке")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, trip.StatusCompleted, tr.Status)
			require.NotNil(t, tr.FinishedAt)
			assert.True(t, now.Equal(*tr.FinishedAt))
		})
	}
}
