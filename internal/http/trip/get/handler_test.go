package get_test

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
	"github.com/505Eclypsia/template/internal/http/trip/get"
	"github.com/505Eclypsia/template/internal/model"
)

func doGet(uc *MockuseCase, id uuid.UUID) *httptest.ResponseRecorder {
	h := get.New(uc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+id.String(), nil)
	rec := httptest.NewRecorder()

	h.Get(rec, req, id)
	return rec
}

func TestHandler_Get_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockuseCase(ctrl)

	finished := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tr := &model.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  59.93,
		StartLongitude: 30.31,
		EndLatitude:    59.92,
		EndLongitude:   30.36,
		Price:          1000,
		Status:         domain.StatusCompleted,
		StartedAt:      finished.Add(-time.Hour),
		FinishedAt:     &finished,
	}
	uc.EXPECT().Get(gomock.Any(), tr.ID).Return(tr, nil)

	rec := doGet(uc, tr.ID)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got api.Trip
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, tr.ID, got.Id)
	assert.Equal(t, tr.UserID, got.UserId)
	assert.Equal(t, tr.DriverID, got.DriverId)
	assert.Equal(t, api.TripStatus("completed"), got.Status)
	assert.EqualValues(t, 1000, got.Price)
	assert.InDelta(t, 59.93, got.StartPoint.Latitude, 1e-9)
	require.NotNil(t, got.FinishedAt)
	assert.True(t, finished.Equal(*got.FinishedAt))
}

func TestHandler_Get_Errors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"not found", domain.ErrTripNotFound, http.StatusNotFound, "trip_not_found"},
		{"unexpected", errors.New("db is down"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockuseCase(ctrl)
			id := uuid.New()
			uc.EXPECT().Get(gomock.Any(), id).Return(nil, tt.err)

			rec := doGet(uc, id)

			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

			var p api.Problem
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
			assert.Equal(t, tt.wantCode, p.Code)
			if tt.wantStatus == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "db is down")
			}
		})
	}
}
