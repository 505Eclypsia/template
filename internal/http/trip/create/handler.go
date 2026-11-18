package create

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	dE "github.com/505Eclypsia/template/internal/http/domainerror"
)

type Handler struct {
	useCase useCase
	logger  *slog.Logger
}

func New(useCase useCase, logger *slog.Logger) *Handler {
	return &Handler{useCase: useCase, logger: logger}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip", err)
		return
	}
	var req api.CreateTripJSONRequestBody
	if err := json.Unmarshal(rawBody, &req); err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip", &domain.ValidationError{
			Field:   "body",
			Message: "invalid request body: " + err.Error(),
		})
		return
	}
	tripModel := convertJsonToModel(&req)

	var idempotencyKeyID *uuid.UUID
	if params.IdempotencyKey != nil {
		key := *params.IdempotencyKey
		idempotencyKeyID = &key
	}

	trip, isNew, err := h.useCase.Create(r.Context(), &tripModel, idempotencyKeyID)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if isNew {
		w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	resp := convertModelToJson(trip)
	_ = json.NewEncoder(w).Encode(resp)
}
