package create_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/http/tripposition/create"
	"github.com/505Eclypsia/template/internal/model"
)

type stubUseCase struct {
	fn func(ctx context.Context, pos *model.TripPosition) (*model.TripPosition, error)
}

func (s *stubUseCase) Create(ctx context.Context, pos *model.TripPosition) (*model.TripPosition, error) {
	return s.fn(ctx, pos)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandler_CreateTripPosition_Success(t *testing.T) {
	tripID := uuid.New()
	recordedAt := time.Date(2026, 8, 24, 12, 0, 5, 0, time.UTC)

	uc := &stubUseCase{
		fn: func(_ context.Context, pos *model.TripPosition) (*model.TripPosition, error) {
			assert.Equal(t, tripID, pos.TripID)
			assert.InDelta(t, 59.935, pos.Latitude, 1e-9)
			assert.InDelta(t, 30.335, pos.Longitude, 1e-9)
			out := *pos
			out.ID = 42
			return &out, nil
		},
	}

	h := create.New(uc, discardLogger())
	body := `{"coordinates":{"latitude":59.935,"longitude":30.335},"recorded_at":"2026-08-24T12:00:05Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.CreateTripPosition(rec, req, tripID)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got api.TripPosition
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.EqualValues(t, 42, got.Id)
	assert.Equal(t, tripID, got.TripId)
	assert.True(t, recordedAt.Equal(got.RecordedAt))
}

func TestHandler_CreateTripPosition_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		ucErr      error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "invalid json",
			body:       `{bad`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "validation error",
			body:       `{"coordinates":{"latitude":95,"longitude":30},"recorded_at":"2026-08-24T12:00:05Z"}`,
			ucErr:      &domain.ValidationError{Field: "coordinates.latitude", Message: "out of range"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
		},
		{
			name:       "trip not found",
			body:       `{"coordinates":{"latitude":59,"longitude":30},"recorded_at":"2026-08-24T12:00:05Z"}`,
			ucErr:      domain.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
		},
		{
			name:       "trip completed",
			body:       `{"coordinates":{"latitude":59,"longitude":30},"recorded_at":"2026-08-24T12:00:05Z"}`,
			ucErr:      domain.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &stubUseCase{
				fn: func(_ context.Context, _ *model.TripPosition) (*model.TripPosition, error) {
					return nil, tt.ucErr
				},
			}
			h := create.New(uc, discardLogger())
			tripID := uuid.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/positions", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.CreateTripPosition(rec, req, tripID)

			require.Equal(t, tt.wantStatus, rec.Code)
			var p api.Problem
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
			assert.Equal(t, tt.wantCode, p.Code)
		})
	}
}
