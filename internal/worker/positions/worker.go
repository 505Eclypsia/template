package positions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/505Eclypsia/template/internal/model"
)

const (
	CheckQueued    = "queued"
	CheckDuplicate = "duplicate"
	CheckDropped   = "dropped"
)

const (
	RequestSuccess   = "success"
	RequestError     = "error"
	RequestCancelled = "cancelled"
)

type tripRepository interface {
	ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
}

type pusher interface {
	RequestPosition(ctx context.Context, driverID, tripID uuid.UUID) error
}

type metrics interface {
	IncStaleCheck(ctx context.Context, result string)
	IncPositionRequest(ctx context.Context, result string)
	RecordPositionRequestDuration(ctx context.Context, d time.Duration, result string)
	AddQueueSize(ctx context.Context, delta int64)
	AddActiveWorkers(ctx context.Context, delta int64)
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type timeTicker struct{ t *time.Ticker }

func NewTicker(d time.Duration) Ticker { return &timeTicker{t: time.NewTicker(d)} }

func (t *timeTicker) C() <-chan time.Time { return t.t.C }
func (t *timeTicker) Stop()               { t.t.Stop() }

type Config struct {
	StaleAfter     time.Duration // POSITION_STALE_AFTER
	Workers        int           // POSITION_WORKERS
	QueueSize      int           // POSITION_QUEUE_SIZE
	RequestTimeout time.Duration // PUSH_REQUEST_TIMEOUT
}

func (c Config) validate() error {
	switch {
	case c.StaleAfter <= 0:
		return errors.New("stale after must be positive")
	case c.Workers < 1:
		return errors.New("workers must be at least 1")
	case c.QueueSize < 1:
		return errors.New("queue size must be at least 1")
	case c.RequestTimeout <= 0:
		return errors.New("request timeout must be positive")
	}
	return nil
}

type task struct {
	tripID   uuid.UUID
	driverID uuid.UUID
	gen      uint64
	link     trace.Link
}

type Worker struct {
	cfg     Config
	repo    tripRepository
	pusher  pusher
	metrics metrics
	ticker  Ticker
	logger  *slog.Logger
	tracer  trace.Tracer

	queue chan task

	mu      sync.Mutex
	pending map[uuid.UUID]uint64
	seq     uint64

	started    bool
	scanCancel context.CancelFunc
	workCancel context.CancelFunc
	scanWG     sync.WaitGroup
	workersWG  sync.WaitGroup
}

func New(
	cfg Config,
	repo tripRepository,
	pusher pusher,
	metrics metrics,
	ticker Ticker,
	logger *slog.Logger,
) (*Worker, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("positions worker config: %w", err)
	}
	return &Worker{
		cfg:     cfg,
		repo:    repo,
		pusher:  pusher,
		metrics: metrics,
		ticker:  ticker,
		logger:  logger,
		tracer:  otel.Tracer("trip-service/worker/positions"),
		queue:   make(chan task, cfg.QueueSize),
		pending: make(map[uuid.UUID]uint64),
	}, nil
}

func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.started {
		w.mu.Unlock()
		return
	}
	w.started = true
	w.mu.Unlock()

	workCtx, workCancel := context.WithCancel(context.WithoutCancel(ctx))
	w.workCancel = workCancel
	for i := 0; i < w.cfg.Workers; i++ {
		w.workersWG.Add(1)
		go w.runWorker(workCtx)
	}

	scanCtx, scanCancel := context.WithCancel(ctx)
	w.scanCancel = scanCancel
	w.scanWG.Add(1)
	go w.runScanner(scanCtx)

	w.logger.Info("positions worker started",
		"workers", w.cfg.Workers,
		"queue_size", w.cfg.QueueSize,
		"stale_after", w.cfg.StaleAfter,
	)
}

func (w *Worker) Stop(ctx context.Context) error {
	w.mu.Lock()
	started := w.started
	w.mu.Unlock()
	if !started {
		return nil
	}

	w.scanCancel()
	w.scanWG.Wait()
	w.ticker.Stop()

	close(w.queue)

	done := make(chan struct{})
	go func() {
		w.workersWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		w.workCancel()
		w.logger.Info("positions worker stopped")
		return nil
	case <-ctx.Done():
		w.workCancel()
		<-done
		w.logger.Warn("positions worker stopped with unfinished requests", "error", ctx.Err())
		return fmt.Errorf("positions worker stopped before queue drained: %w", ctx.Err())
	}
}

func (w *Worker) Cancel(tripID uuid.UUID) {
	w.mu.Lock()
	delete(w.pending, tripID)
	w.mu.Unlock()
}

func (w *Worker) runScanner(ctx context.Context) {
	defer w.scanWG.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.ticker.C():
			w.scan(ctx)
		}
	}
}

func (w *Worker) scan(ctx context.Context) {
	ctx, span := w.tracer.Start(ctx, "positions.scan")
	defer span.End()

	trips, err := w.repo.ListStaleTrips(ctx, w.cfg.StaleAfter)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		if ctx.Err() == nil {
			w.logger.Error("list stale trips", "error", err)
		}
		return
	}

	link := trace.LinkFromContext(ctx)
	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}
		w.enqueue(ctx, trip, link)
	}
}

func (w *Worker) enqueue(ctx context.Context, trip model.StaleTrip, link trace.Link) {
	gen, ok := w.markPending(trip.TripID)
	if !ok {
		w.metrics.IncStaleCheck(ctx, CheckDuplicate)
		return
	}

	t := task{tripID: trip.TripID, driverID: trip.DriverID, gen: gen, link: link}

	select {
	case w.queue <- t:
		w.metrics.AddQueueSize(ctx, 1)
		w.metrics.IncStaleCheck(ctx, CheckQueued)
	default:
		w.release(trip.TripID, gen)
		w.metrics.IncStaleCheck(ctx, CheckDropped)
		w.logger.Warn("position queue is full, trip dropped",
			"trip_id", trip.TripID, "queue_size", w.cfg.QueueSize)
	}
}

func (w *Worker) runWorker(ctx context.Context) {
	defer w.workersWG.Done()

	for t := range w.queue {
		w.metrics.AddQueueSize(ctx, -1)
		w.process(ctx, t)
	}
}

func (w *Worker) process(ctx context.Context, t task) {
	if !w.isCurrent(t.tripID, t.gen) {
		w.metrics.IncPositionRequest(ctx, RequestCancelled)
		return
	}
	defer w.release(t.tripID, t.gen)

	w.metrics.AddActiveWorkers(ctx, 1)
	defer w.metrics.AddActiveWorkers(ctx, -1)

	ctx, span := w.tracer.Start(ctx, "positions.request", trace.WithLinks(t.link))
	defer span.End()

	reqCtx, cancel := context.WithTimeout(ctx, w.cfg.RequestTimeout)
	defer cancel()

	start := time.Now()
	err := w.pusher.RequestPosition(reqCtx, t.driverID, t.tripID)
	elapsed := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		w.metrics.RecordPositionRequestDuration(ctx, elapsed, RequestError)
		w.metrics.IncPositionRequest(ctx, RequestError)
		w.logger.Warn("request position failed", "trip_id", t.tripID, "error", err)
		return
	}
	w.metrics.RecordPositionRequestDuration(ctx, elapsed, RequestSuccess)
	w.metrics.IncPositionRequest(ctx, RequestSuccess)
}

func (w *Worker) markPending(tripID uuid.UUID) (uint64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.pending[tripID]; exists {
		return 0, false
	}
	w.seq++
	w.pending[tripID] = w.seq
	return w.seq, true
}

func (w *Worker) isCurrent(tripID uuid.UUID, gen uint64) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pending[tripID] == gen
}

func (w *Worker) release(tripID uuid.UUID, gen uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending[tripID] == gen {
		delete(w.pending, tripID)
	}
}
