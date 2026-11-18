package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	"github.com/505Eclypsia/template/internal/http/trip/create"
	"github.com/505Eclypsia/template/internal/model"
)

const problemContentType = "application/problem+json"

func validBody(userID, driverID uuid.UUID) string {
	return `{
		"user_id": "` + userID.String() + `",
		"driver_id": "` + driverID.String() + `",
		"start_point": {"latitude": 59.93, "longitude": 30.31},
		"end_point": {"latitude": 59.92, "longitude": 30.36},
		"price": 1000
	}`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func doCreate(t *testing.T, uc *MockuseCase, body string) *httptest.ResponseRecorder {
	t.Helper()
	return doCreateWithLogger(t, uc, discardLogger(), body)
}

func doCreateWithLogger(t *testing.T, uc *MockuseCase, logger *slog.Logger, body string) *httptest.ResponseRecorder {
	t.Helper()

	h := create.New(uc, logger)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Create(rec, req, api.CreateTripParams{})
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) api.Problem {
	t.Helper()

	var p api.Problem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&p))
	return p
}

func TestHandler_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockuseCase(ctrl)

	userID, driverID, tripID := uuid.New(), uuid.New(), uuid.New()

	uc.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Nil()).
		DoAndReturn(func(_ any, in *model.Trip, _ *uuid.UUID) (*model.Trip, bool, error) {
			assert.Equal(t, userID, in.UserID)
			assert.Equal(t, driverID, in.DriverID)
			assert.InDelta(t, 59.93, in.StartLatitude, 1e-9)
			assert.InDelta(t, 30.36, in.EndLongitude, 1e-9)
			assert.EqualValues(t, 1000, in.Price)

			out := *in
			out.ID = tripID
			out.Status = domain.StatusActive
			return &out, true, nil
		})

	rec := doCreate(t, uc, validBody(userID, driverID))

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "/api/v1/trips/"+tripID.String(), rec.Header().Get("Location"))

	var got api.Trip
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, tripID, got.Id)
	assert.Equal(t, userID, got.UserId)
	assert.Equal(t, driverID, got.DriverId)
	assert.Equal(t, api.TripStatus("active"), got.Status)
	assert.EqualValues(t, 1000, got.Price)
}

func TestHandler_Create_IdempotentReplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockuseCase(ctrl)

	existing := &model.Trip{ID: uuid.New(), Status: domain.StatusActive, Price: 1000}
	uc.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(existing, false, nil)

	rec := doCreate(t, uc, validBody(uuid.New(), uuid.New()))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Location"))

	var got api.Trip
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&got))
	assert.Equal(t, existing.ID, got.Id)
}

func TestHandler_Create_InvalidBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "broken json", body: `{"user_id": `},
		{name: "not a json", body: `hello`},
		{name: "empty body", body: ``},
		{name: "invalid uuid", body: `{"user_id":"not-a-uuid","driver_id":"` + uuid.NewString() + `","start_point":{"latitude":1,"longitude":1},"end_point":{"latitude":1,"longitude":1},"price":1}`},
		{name: "wrong field type", body: `{"price": "expensive"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockuseCase(ctrl)

			rec := doCreate(t, uc, tt.body)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, problemContentType, rec.Header().Get("Content-Type"))

			p := decodeProblem(t, rec)
			assert.Equal(t, "invalid_request", p.Code)
			assert.EqualValues(t, http.StatusBadRequest, p.Status)
		})
	}
}

func TestHandler_Create_UseCaseErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", &domain.ValidationError{Field: "price", Message: "must be positive"}, http.StatusBadRequest, "invalid_request"},
		{"driver busy", domain.ErrDriverBusy, http.StatusConflict, "driver_busy"},
		{"idempotency conflict", domain.ErrIdempotencyConflict, http.StatusConflict, "idempotency_conflict"},
		{"unexpected", errors.New("pq: connection refused to 10.0.0.5"), http.StatusInternalServerError, "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			uc := NewMockuseCase(ctrl)
			uc.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(nil, false, tt.err)

			rec := doCreate(t, uc, validBody(uuid.New(), uuid.New()))

			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, problemContentType, rec.Header().Get("Content-Type"))
			assert.Empty(t, rec.Header().Get("Location"))

			p := decodeProblem(t, rec)
			assert.Equal(t, tt.wantCode, p.Code)

			if tt.wantStatus == http.StatusInternalServerError {
				assert.NotContains(t, rec.Body.String(), "10.0.0.5")
			}
		})
	}
}

func TestHandler_Create_LogsOriginalError(t *testing.T) {
	ctrl := gomock.NewController(t)
	uc := NewMockuseCase(ctrl)
	uc.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, false, errors.New("pq: connection refused to 10.0.0.5"))

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	rec := doCreateWithLogger(t, uc, logger, validBody(uuid.New(), uuid.New()))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "10.0.0.5")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	assert.Equal(t, "ERROR", entry["level"])
	assert.Contains(t, entry["error"], "10.0.0.5")
}
