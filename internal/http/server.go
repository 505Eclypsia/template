package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	domain "github.com/505Eclypsia/template/internal/domain/trip"
	"github.com/505Eclypsia/template/internal/generated/api"
	dE "github.com/505Eclypsia/template/internal/http/domainerror"
	tripCreate "github.com/505Eclypsia/template/internal/http/trip/create"
	tripFinish "github.com/505Eclypsia/template/internal/http/trip/finish"
	tripGet "github.com/505Eclypsia/template/internal/http/trip/get"
	tripPosCreate "github.com/505Eclypsia/template/internal/http/tripposition/create"
	tripPosGet "github.com/505Eclypsia/template/internal/http/tripposition/get"
)

type pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	createHandler    *tripCreate.Handler
	getHandler       *tripGet.Handler
	finishHandler    *tripFinish.Handler
	createPosHandler *tripPosCreate.Handler
	listPosHandler   *tripPosGet.Handler
	db               pinger
	readyTimeout     time.Duration
}

func NewServer(
	createH *tripCreate.Handler,
	getH *tripGet.Handler,
	finishH *tripFinish.Handler,
	createPosH *tripPosCreate.Handler,
	listPosH *tripPosGet.Handler,
	db pinger,
	readyTimeout time.Duration,
) *Server {
	return &Server{
		createHandler:    createH,
		getHandler:       getH,
		finishHandler:    finishH,
		createPosHandler: createPosH,
		listPosHandler:   listPosH,
		db:               db,
		readyTimeout:     readyTimeout,
	}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.createHandler.Create(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.getHandler.Get(w, r, tripId)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.finishHandler.Finish(w, r, tripId)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: "ok"})
}

func (s *Server) HandleAPIError(w http.ResponseWriter, r *http.Request, err error) {
	dE.WriteProblem(w, r, &domain.ValidationError{Field: "request", Message: err.Error()})
}

const defaultReadyTimeout = 2 * time.Second

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	timeout := s.readyTimeout
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}
	pingCtx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	if err := s.db.Ping(pingCtx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: "unavailable"})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: "ok"})
}

func (s *Server) CreateTripPosition(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.createPosHandler.CreateTripPosition(w, r, tripId)
}

func (s *Server) ListTripPositions(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.listPosHandler.ListTripPositions(w, r, tripId)
}
