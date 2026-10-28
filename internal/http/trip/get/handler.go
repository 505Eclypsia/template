package get

import (
	"encoding/json"
	"net/http"

	"github.com/505Eclypsia/template/internal/generated/api"
	dE "github.com/505Eclypsia/template/internal/http/domainerror"
)

type Handler struct {
	useCase useCase
}

func New(useCase useCase) *Handler {
	return &Handler{useCase: useCase}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	id := tripId

	tripModel, err := h.useCase.Get(r.Context(), id)
	if err != nil {
		dE.WriteProblem(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(convertModelToJSON(tripModel))
}
