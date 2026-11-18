package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/505Eclypsia/template/internal/config"
	"github.com/505Eclypsia/template/internal/gateway/push"
	"github.com/505Eclypsia/template/internal/generated/api"
	appHTTP "github.com/505Eclypsia/template/internal/http"
	appMiddleware "github.com/505Eclypsia/template/internal/http/middleware"
	tripCreateHTTP "github.com/505Eclypsia/template/internal/http/trip/create"
	tripFinishHTTP "github.com/505Eclypsia/template/internal/http/trip/finish"
	tripGetHTTP "github.com/505Eclypsia/template/internal/http/trip/get"
	tripPosCreateHTTP "github.com/505Eclypsia/template/internal/http/tripposition/create"
	tripPosGetHTTP "github.com/505Eclypsia/template/internal/http/tripposition/get"
	"github.com/505Eclypsia/template/internal/infra/postgres"
	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
	"github.com/505Eclypsia/template/internal/infra/telemetry"
	tripRepo "github.com/505Eclypsia/template/internal/repository/trip"
	tripCreateUC "github.com/505Eclypsia/template/internal/usecase/trip/create"
	tripFinishUC "github.com/505Eclypsia/template/internal/usecase/trip/finish"
	tripGetUC "github.com/505Eclypsia/template/internal/usecase/trip/get"
	tripPosCreateUC "github.com/505Eclypsia/template/internal/usecase/tripposition/create"
	tripPosGetUC "github.com/505Eclypsia/template/internal/usecase/tripposition/get"
	"github.com/505Eclypsia/template/internal/worker/positions"
)

type Container struct {
	Server   *http.Server
	Shutdown func(context.Context) error
	Logger   *slog.Logger

	PositionsWorker *positions.Worker
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger, teleShutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry setup: %w", err)
	}
	if err := telemetry.InitBusinessMetrics(); err != nil {
		_ = teleShutdown(ctx)
		return nil, fmt.Errorf("business metrics setup: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		_ = teleShutdown(ctx)
		return nil, fmt.Errorf("postgres setup: %w", err)
	}

	txManager := transaction.NewTxManager(pool)
	repo := tripRepo.NewTripRepo(pool, cfg.DatabaseQueryTimeout)
	pushClient := push.New(cfg.PushHTTPURL, cfg.PushRequestTimeout)

	positionsWorker, err := positions.New(
		positions.Config{
			StaleAfter:     cfg.PositionStaleAfter,
			Workers:        int(cfg.PositionWorkers),
			QueueSize:      int(cfg.PositionQueueSize),
			RequestTimeout: cfg.PushRequestTimeout,
		},
		repo,
		pushClient,
		telemetry.Metrics,
		positions.NewTicker(cfg.PositionCheckInterval),
		logger,
	)
	if err != nil {
		pushClient.Close()
		pool.Close()
		_ = teleShutdown(ctx)
		return nil, fmt.Errorf("positions worker setup: %w", err)
	}

	createUC := tripCreateUC.New(repo, txManager, telemetry.Metrics)
	getUC := tripGetUC.New(repo)
	finishUC := tripFinishUC.New(repo, txManager, telemetry.Metrics)
	createPosUC := tripPosCreateUC.New(repo, txManager, positionsWorker, telemetry.Metrics)
	listPosUC := tripPosGetUC.New(repo)

	createH := tripCreateHTTP.New(createUC, logger)
	getH := tripGetHTTP.New(getUC, logger)
	finishH := tripFinishHTTP.New(finishUC, logger)
	createPosH := tripPosCreateHTTP.New(createPosUC, logger)
	listPosH := tripPosGetHTTP.New(listPosUC, logger)

	serverImpl := appHTTP.NewServer(createH, getH, finishH, createPosH, listPosH, pool, cfg.DatabaseConnectTimeout)

	r := chi.NewRouter()
	appMiddleware.Chain(r, logger)
	api.HandlerWithOptions(serverImpl, api.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: serverImpl.HandleAPIError,
	})

	positionsWorker.Start(ctx)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownAll := func(sCtx context.Context) error {
		var errs []error

		if err := httpServer.Shutdown(sCtx); err != nil {
			errs = append(errs, fmt.Errorf("http server: %w", err))
		}
		if err := positionsWorker.Stop(sCtx); err != nil {
			errs = append(errs, fmt.Errorf("positions worker: %w", err))
		}
		pushClient.Close()
		pool.Close()
		if err := teleShutdown(sCtx); err != nil {
			errs = append(errs, fmt.Errorf("telemetry: %w", err))
		}
		return errors.Join(errs...)
	}

	return &Container{
		Server:          httpServer,
		Shutdown:        shutdownAll,
		Logger:          logger,
		PositionsWorker: positionsWorker,
	}, nil
}
