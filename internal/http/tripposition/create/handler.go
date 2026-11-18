package create

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

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

func (h *Handler) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip position", err)
		return
	}
	var req api.CreateTripPositionJSONRequestBody
	if err := json.Unmarshal(rawBody, &req); err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip position", &domain.ValidationError{
			Field:   "body",
			Message: "invalid request body: " + err.Error(),
		})
		return
	}
	tripPositionModel := convertJSONToModel(&req, tripID)

	createdTripPositionModel, err := h.useCase.Create(r.Context(), &tripPositionModel)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "create trip position", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := convertModelToJSON(createdTripPositionModel)
	_ = json.NewEncoder(w).Encode(resp)
}
