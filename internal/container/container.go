package container

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/505Eclypsia/template/internal/config"
	"github.com/505Eclypsia/template/internal/generated/api"
	appHTTP "github.com/505Eclypsia/template/internal/http"
	appMiddleware "github.com/505Eclypsia/template/internal/http/middleware"
	tripCreateHTTP "github.com/505Eclypsia/template/internal/http/trip/create"
	tripFinishHTTP "github.com/505Eclypsia/template/internal/http/trip/finish"
	tripGetHTTP "github.com/505Eclypsia/template/internal/http/trip/get"
	"github.com/505Eclypsia/template/internal/infra/postgres"
	"github.com/505Eclypsia/template/internal/infra/postgres/transaction"
	"github.com/505Eclypsia/template/internal/infra/telemetry"
	tripRepo "github.com/505Eclypsia/template/internal/repository/trip"
	tripCreateUC "github.com/505Eclypsia/template/internal/usecase/trip/create"
	tripFinishUC "github.com/505Eclypsia/template/internal/usecase/trip/finish"
	tripGetUC "github.com/505Eclypsia/template/internal/usecase/trip/get"
)

type Container struct {
	Server   *http.Server
	Shutdown func(context.Context) error
	Logger   *slog.Logger
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	logger, teleShutdown, err := telemetry.Setup(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry setup: %w", err)
	}
	if err := telemetry.InitBusinessMetrics(); err != nil {
		return nil, fmt.Errorf("business metrics setup: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres setup: %w", err)
	}

	txManager := transaction.NewTxManager(pool)
	repo := tripRepo.NewTripRepo(pool, cfg.DatabaseQueryTimeout)

	createUC := tripCreateUC.New(repo, txManager, telemetry.Metrics)
	getUC := tripGetUC.New(repo)
	finishUC := tripFinishUC.New(repo, txManager, telemetry.Metrics)

	createH := tripCreateHTTP.New(createUC, logger)
	getH := tripGetHTTP.New(getUC, logger)
	finishH := tripFinishHTTP.New(finishUC, logger)

	serverImpl := appHTTP.NewServer(createH, getH, finishH, pool, cfg.DatabaseConnectTimeout)

	r := chi.NewRouter()
	appMiddleware.Chain(r, logger)
	api.HandlerWithOptions(serverImpl, api.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: serverImpl.HandleAPIError,
	})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	shutdownAll := func(sCtx context.Context) error {
		_ = httpServer.Shutdown(sCtx)
		pool.Close()
		_ = teleShutdown(sCtx)
		return nil
	}

	return &Container{
		Server:   httpServer,
		Shutdown: shutdownAll,
		Logger:   logger,
	}, nil
}
