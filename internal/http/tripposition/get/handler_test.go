package get_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/http/tripposition/get"
	"github.com/505Eclypsia/template/internal/model"
)

type stubUseCase struct {
	fn func(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error)
}

func (s *stubUseCase) GetTripPositions(ctx context.Context, tripID uuid.UUID) ([]model.TripPosition, error) {
	return s.fn(ctx, tripID)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandler_ListTripPositions_EmptyReturnsArray(t *testing.T) {
	tripID := uuid.New()
	uc := &stubUseCase{
		fn: func(_ context.Context, _ uuid.UUID) ([]model.TripPosition, error) {
			return nil, nil
		},
	}
	h := get.New(uc, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	rec := httptest.NewRecorder()

	h.ListTripPositions(rec, req, tripID)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"positions":[]}`, rec.Body.String())
}

func TestHandler_ListTripPositions_Success(t *testing.T) {
	tripID := uuid.New()
	ts := time.Date(2026, 8, 24, 12, 0, 5, 0, time.UTC)
	uc := &stubUseCase{
		fn: func(_ context.Context, _ uuid.UUID) ([]model.TripPosition, error) {
			return []model.TripPosition{
				{ID: 1, TripID: tripID, Latitude: 59.93, Longitude: 30.31, RecordedAt: ts},
			}, nil
		},
	}
	h := get.New(uc, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	rec := httptest.NewRecorder()

	h.ListTripPositions(rec, req, tripID)

	require.Equal(t, http.StatusOK, rec.Code)
	var got api.TripPositionsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	require.Len(t, got.Positions, 1)
	assert.EqualValues(t, 1, got.Positions[0].Id)
}

func TestHandler_ListTripPositions_NotFound(t *testing.T) {
	tripID := uuid.New()
	uc := &stubUseCase{
		fn: func(_ context.Context, _ uuid.UUID) ([]model.TripPosition, error) {
			return nil, domain.ErrTripNotFound
		},
	}
	h := get.New(uc, discardLogger())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+tripID.String()+"/positions", nil)
	rec := httptest.NewRecorder()

	h.ListTripPositions(rec, req, tripID)

	require.Equal(t, http.StatusNotFound, rec.Code)
	var p api.Problem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
	assert.Equal(t, "trip_not_found", p.Code)
}
