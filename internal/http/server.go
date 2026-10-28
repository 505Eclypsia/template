package http

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/505Eclypsia/template/internal/generated/api"
	tripCreate "github.com/505Eclypsia/template/internal/http/trip/create"
	tripFinish "github.com/505Eclypsia/template/internal/http/trip/finish"
	tripGet "github.com/505Eclypsia/template/internal/http/trip/get"
)

type Server struct {
	createHandler *tripCreate.Handler
	getHandler    *tripGet.Handler
	finishHandler *tripFinish.Handler
	pool          *pgxpool.Pool
}

func NewServer(
	createH *tripCreate.Handler,
	getH *tripGet.Handler,
	finishH *tripFinish.Handler,
	pool *pgxpool.Pool,
) *Server {
	return &Server{
		createHandler: createH,
		getHandler:    getH,
		finishHandler: finishH,
		pool:          pool,
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

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: "unavailable"})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: "ok"})
}

func (s *Server) CreateTripPosition(w http.ResponseWriter, _ *http.Request, _ api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}

func (s *Server) ListTripPositions(w http.ResponseWriter, _ *http.Request, _ api.TripId) {
	w.WriteHeader(http.StatusNotImplemented)
}
