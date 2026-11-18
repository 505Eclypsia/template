package get

import (
	"encoding/json"
	"log/slog"
	"net/http"

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

func (h *Handler) ListTripPositions(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripPositions, err := h.useCase.GetTripPositions(r.Context(), tripID)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "get trip positions", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(convertModelsToJSON(tripPositions))
}
