package finish

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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

func (h *Handler) Finish(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	id := tripId
	now := time.Now()

	tripModel, err := h.useCase.Finish(r.Context(), id, now)
	if err != nil {
		dE.WriteAndLog(w, r, h.logger, "finish trip", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(convertModelToJSON(tripModel))
}
