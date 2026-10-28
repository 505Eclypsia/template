package create

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/505Eclypsia/template/internal/generated/api"
	dE "github.com/505Eclypsia/template/internal/http/domainerror"
)

type Handler struct {
	useCase useCase
}

func New(useCase useCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		dE.WriteProblem(w, r, err)
		return
	}
	var req api.CreateTripJSONRequestBody
	if err := json.Unmarshal(rawBody, &req); err != nil {
		dE.WriteProblem(w, r, err)
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
		dE.WriteProblem(w, r, err)
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
