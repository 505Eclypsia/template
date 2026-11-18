package finish_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/http/trip/finish"
	"github.com/505Eclypsia/template/internal/model"
)

func doFinish(uc *MockuseCase, id uuid.UUID) *httptest.ResponseRecorder {
	h := finish.New(uc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+id.String()+"/finish", nil)
	rec := httptest.NewRecorder()

	h.Finish(rec, req, id)
	return rec
}

func TestHandler_Finish_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockuseCase(ctrl)

	id := uuid.New()
	var passedAt time.Time

	uc.EXPECT().
		Finish(gomock.Any(), id, gomock.Any()).
		DoAndReturn(func(_ any, id uuid.UUID, at time.Time) (*model.Trip, error) {
			passedAt = at
			return &model.Trip{
				ID:         id,
				UserID:     uuid.New(),
				DriverID:   uuid.New(),
				Price:      1000,
				Status:     domain.StatusCompleted,
				StartedAt:  at.Add(-time.Hour),
				FinishedAt: &at,
			}, nil
		})

	rec := doFinish(uc, id)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got api.Trip
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, id, got.Id)
	assert.Equal(t, api.TripStatus("completed"), got.Status)
	require.NotNil(t, got.FinishedAt)
	assert.True(t, passedAt.Equal(*got.FinishedAt), "finished_at в ответе должен совпадать со временем, переданным в usecase")
}

func TestHandler_Finish_Errors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", domain.ErrTripNotFound, http.StatusNotFound, "trip_not_found"},
		{"already completed", domain.ErrTripCompleted, http.StatusConflict, "trip_completed"},
		{"unexpected", errors.New("deadlock detected"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockuseCase(ctrl)
			id := uuid.New()
			uc.EXPECT().Finish(gomock.Any(), id, gomock.Any()).Return(nil, tt.err)

			rec := doFinish(uc, id)

			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

			var p api.Problem
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
			assert.Equal(t, tt.wantCode, p.Code)
			if tt.wantStatus == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "deadlock")
			}
		})
	}
}
