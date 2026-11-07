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

var ErrIdempotencyConflict = errors.New("idempotency conflict")

func computeHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

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

func (h *Handler) HandleAPIError(w http.ResponseWriter, r *http.Request, err error) {
	writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", err.Error())
}

func validateCreateTripRequest(req *api.CreateTripJSONRequestBody) bool {
	if req.UserId == uuid.Nil || req.DriverId == uuid.Nil {
		return false
	}
	if req.Price < 0 {
		return false
	}
	if req.StartPoint.Latitude < -90 || req.StartPoint.Latitude > 90 ||
		req.StartPoint.Longitude < -180 || req.StartPoint.Longitude > 180 ||
		req.EndPoint.Latitude < -90 || req.EndPoint.Latitude > 90 ||
		req.EndPoint.Longitude < -180 || req.EndPoint.Longitude > 180 {
		return false
	}
	return true
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

	if !validateCreateTripRequest(&req) {
		writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Coordinates or price out of range")
		return
	}

	keyHeader := r.Header.Get("Idempotency-Key")
	var idempotencyKeyID *uuid.UUID
	if keyHeader != "" {
		parsedKey, err := uuid.Parse(keyHeader)
		if err != nil {
			writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", "Idempotency-Key must be a valid UUID")
			return
		}
		idempotencyKeyID = &parsedKey
	}

	requestHash := computeHash(rawBody)
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

	var existingTripID *uuid.UUID

	err = h.txManager.Do(r.Context(), func(txCtx context.Context) error {
		if idempotencyKeyID != nil {
			if err := h.repo.LockIdempotencyKey(txCtx, *idempotencyKeyID); err != nil {
				return err
			}

			keyModel, err := h.repo.GetIdempotencyKeyByID(txCtx, *idempotencyKeyID)
			if err == nil {
				if keyModel.RequestHash != requestHash {
					return ErrIdempotencyConflict
				}
				existingTripID = &keyModel.TripID
				return nil
			}
			if !errors.Is(err, db.ErrIdempotencyKeyNotFound) {
				return err
			}
		}

		if err := h.repo.Create(txCtx, trip); err != nil {
			return err
		}
		if err := h.repo.AddTripStatusHistory(txCtx, history); err != nil {
			return err
		}

		if idempotencyKeyID != nil {
			return h.repo.AddTripIdempotencyKey(txCtx, models.TripIdempotencyKey{
				ID:          *idempotencyKeyID,
				RequestHash: requestHash,
				TripID:      tripID,
			})
		}
		return nil
	})

	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) {
			writeProblem(w, r, http.StatusConflict, "idempotency_conflict", "Idempotency conflict", "Idempotency-Key was already used with a different request body")
			return
		}
		if errors.Is(err, db.ErrDriverBusy) {
			writeProblem(w, r, http.StatusConflict, "driver_busy", "Driver busy", "Driver already has an active trip")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to create trip")
		return
	}

	if existingTripID != nil {
		h.GetTrip(w, r, *existingTripID)
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

	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripUUID := uuid.UUID(tripId)
	tripModel, err := h.repo.GetTripByID(r.Context(), tripUUID)
	if err != nil {
		if errors.Is(err, db.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "There is no trip with such ID")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get trip")
		return
	}

	resp := api.Trip{
		Id:         tripModel.ID,
		UserId:     tripModel.UserID,
		DriverId:   tripModel.DriverID,
		StartPoint: api.Coordinates{Latitude: tripModel.StartLatitude, Longitude: tripModel.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: tripModel.EndLatitude, Longitude: tripModel.EndLongitude},
		Price:      tripModel.Price,
		Status:     api.TripStatus(tripModel.Status),
		StartedAt:  tripModel.StartedAt,
		FinishedAt: tripModel.FinishedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripUUID := uuid.UUID(tripId)
	now := time.Now()
	fromStatus := "active"
	reason := "trip finished"

	tripSH := models.TripStatusHistory{
		TripID:     tripUUID,
		FromStatus: &fromStatus,
		ToStatus:   "completed",
		Reason:     &reason,
		ChangedAt:  now,
	}

	err := h.txManager.Do(r.Context(), func(txCtx context.Context) error {
		if err := h.repo.Finish(txCtx, tripUUID); err != nil {
			return err
		}
		return h.repo.AddTripStatusHistory(txCtx, tripSH)
	})

	if err != nil {
		if errors.Is(err, db.ErrTripAlreadyCompleted) {
			writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip already completed", "Trip with such ID is already completed")
			return
		}
		if errors.Is(err, db.ErrTripNotFound) {
			writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "There is no trip with such ID")
			return
		}
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to finish trip")
		return
	}

	trip, err := h.repo.GetTripByID(r.Context(), tripUUID)
	if err != nil {
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal error", "Failed to get trip")
		return
	}

	resp := api.Trip{
		Id:         trip.ID,
		UserId:     trip.UserID,
		DriverId:   trip.DriverID,
		StartPoint: api.Coordinates{Latitude: trip.StartLatitude, Longitude: trip.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: trip.EndLatitude, Longitude: trip.EndLongitude},
		Price:      trip.Price,
		Status:     api.TripStatus("completed"),
		StartedAt:  trip.StartedAt,
		FinishedAt: trip.FinishedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status: "ok",
	})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pingCtx, cancel := context.WithTimeout(r.Context(), h.pool.Config().ConnConfig.ConnectTimeout)
	defer cancel()

	if err := h.pool.Ping(pingCtx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{
			Status: "unavailable",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{
		Status: "ok",
	})
}

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}
