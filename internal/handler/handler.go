package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/api"
	"github.com/505Eclypsia/template/internal/db"
	"github.com/505Eclypsia/template/internal/models"
)

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	prob := api.Problem{
		Type:     "https://tripgo.example/problems/" + code,
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &r.URL.Path,
		Code:     code,
	}

	_ = json.NewEncoder(w).Encode(prob)
}

type Handler struct {
	repo      *db.TripRepo
	txManager db.TxManager
	pool      *pgxpool.Pool
}

func New(repo *db.TripRepo, txManager db.TxManager, pool *pgxpool.Pool) *Handler {
	return &Handler{
		repo:      repo,
		txManager: txManager,
		pool:      pool,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Failed to read body")
		return
	}

	var req api.CreateTripJSONRequestBody
	if err := json.Unmarshal(rawBody, &req); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Failed to parse JSON body")
		return
	}

	if req.Price < 0 ||
		req.StartPoint.Latitude < -90 || req.StartPoint.Latitude > 90 ||
		req.StartPoint.Longitude < -180 || req.StartPoint.Longitude > 180 ||
		req.EndPoint.Latitude < -90 || req.EndPoint.Latitude > 90 ||
		req.EndPoint.Longitude < -180 || req.EndPoint.Longitude > 180 {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Coordinates or price out of range")
		return
	}

	var idempotencyKeyID uuid.UUID
	var requestHash string
	keyHeader := r.Header.Get("Idempotency-Key")
	if keyHeader != "" {
		parsedKey, err := uuid.Parse(keyHeader)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Idempotency-Key must be a valid UUID")
			return
		}
		idempotencyKeyID = parsedKey
		idempotencyKeyModel, err := h.repo.GetIdempotencyKeyByID(r.Context(), idempotencyKeyID)
		requestHash = computeHash(rawBody)
		if err != nil {
			if !errors.Is(err, db.ErrIdempotencyKeyNotFound) {
				writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get idempotency key")
				return
			}
		} else {
			if idempotencyKeyModel.RequestHash != requestHash {
				writeProblem(w, r, 409, "idempotency_conflict", "Idempotency conflict", "Idempotency conflict")
				return
			}
			h.GetTrip(w, r, idempotencyKeyModel.TripID)
			return
		}
	}

	tripID := uuid.New()
	now := time.Now()

	trip := models.Trip{
		ID:             tripID,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  float64(req.StartPoint.Latitude),
		StartLongitude: float64(req.StartPoint.Longitude),
		EndLatitude:    float64(req.EndPoint.Latitude),
		EndLongitude:   float64(req.EndPoint.Longitude),
		Price:          int64(req.Price),
		Status:         "active",
		StartedAt:      now,
	}

	history := models.TripStatusHistory{
		TripID:     tripID,
		FromStatus: nil,
		ToStatus:   "active",
		ChangedAt:  now,
	}

	ctx := r.Context()
	err = h.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := h.repo.Create(txCtx, trip); err != nil {
			return err
		}
		if err := h.repo.AddTripStatusHistory(txCtx, history); err != nil {
			return err
		}

		if keyHeader != "" {
			idempotencyKeyModel := models.TripIdempotencyKey{
				ID:          idempotencyKeyID,
				RequestHash: requestHash,
				TripID:      tripID,
			}
			return h.repo.AddTripIdempotencyKey(txCtx, idempotencyKeyModel)
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, db.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to create trip")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/trips/"+tripID.String())
	w.WriteHeader(http.StatusCreated)

	resp := api.Trip{
		Id:         tripID,
		UserId:     req.UserId,
		DriverId:   req.DriverId,
		StartPoint: req.StartPoint,
		EndPoint:   req.EndPoint,
		Price:      req.Price,
		Status:     api.TripStatus("active"),
		StartedAt:  now,
	}

	json.NewEncoder(w).Encode(resp)
}

func computeHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripUUID := uuid.UUID(tripId)
	if tripUUID == uuid.Nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "tripId must be a valid non-empty UUID")
		return
	}

	tripModel, err := h.repo.GetTripByID(r.Context(), uuid.UUID(tripId))
	if err != nil {
		if errors.Is(err, db.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "There is no trip with such ID")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get trip")
		return
	}

	startPoint := api.Coordinates{Latitude: tripModel.StartLatitude, Longitude: tripModel.StartLongitude}
	endPoint := api.Coordinates{Latitude: tripModel.EndLatitude, Longitude: tripModel.EndLongitude}

	trip := api.Trip{
		Id:         tripModel.ID,
		UserId:     tripModel.UserID,
		DriverId:   tripModel.DriverID,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		Price:      tripModel.Price,
		Status:     api.TripStatus(tripModel.Status),
		StartedAt:  tripModel.StartedAt,
		FinishedAt: tripModel.FinishedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(trip)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripUUID := uuid.UUID(tripId)
	if tripUUID == uuid.Nil {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "tripId must be a valid non-empty UUID")
		return
	}

	fromStatus := "active"
	reason := "trip completed"
	now := time.Now()

	tripSH := models.TripStatusHistory{
		ID:         1,
		TripID:     tripId,
		FromStatus: &fromStatus,
		ToStatus:   "completed",
		Reason:     &reason,
		ChangedAt:  now,
	}

	ctx := r.Context()
	err := h.txManager.Do(ctx, func(txCtx context.Context) error {
		err := h.repo.Finish(txCtx, tripId)
		if err != nil {
			return err
		}
		return h.repo.AddTripStatusHistory(txCtx, tripSH)
	})

	if err != nil {
		if errors.Is(err, db.ErrTripAlreadyCompleted) {
			writeProblem(w, r, 409, "trip_completed", "Trip already completed", "Trip with such ID is already completed")
			return
		}
		if errors.Is(err, db.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "There is no trip with such ID")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get trip")
		return
	}

	trip, err := h.repo.GetTripByID(ctx, tripId)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get trip")
		return
	}

	startPoint := api.Coordinates{Latitude: trip.StartLatitude, Longitude: trip.StartLongitude}
	endPoint := api.Coordinates{Latitude: trip.EndLatitude, Longitude: trip.EndLongitude}

	resp := api.Trip{
		Id:         trip.ID,
		UserId:     trip.UserID,
		DriverId:   trip.DriverID,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		Price:      trip.Price,
		Status:     api.TripStatus("completed"),
		StartedAt:  trip.StartedAt,
		FinishedAt: trip.FinishedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]string{"status": "ok"}
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	err := h.pool.Ping(r.Context())
	w.Header().Set("Content-type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(api.HealthResponse{
			Status: "unavailable",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.HealthResponse{
		Status: "ok",
	})
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}
