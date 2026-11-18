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

func (h *Handler) Get(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	id := tripId

	tripModel, err := h.useCase.Get(r.Context(), id)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "get trip", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(convertModelToJSON(tripModel))
}
