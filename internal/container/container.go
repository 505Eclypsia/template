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
	// 1. Telemetry & slog
	logger, teleShutdown, err := telemetry.Setup(ctx, cfg)
	_ = telemetry.InitBusinessMetrics()
	if err != nil {
		return nil, fmt.Errorf("telemetry setup: %w", err)
	}

	// 2. Postgres
	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres setup: %w", err)
	}

	// 3. Infra & Repos
	txManager := transaction.NewTxManager(pool)
	repo := tripRepo.NewTripRepo(pool)

	// 4. Usecases
	createUC := tripCreateUC.New(repo, txManager)
	getUC := tripGetUC.New(repo)
	finishUC := tripFinishUC.New(repo, txManager)

	// 5. HTTP Handlers
	createH := tripCreateHTTP.New(createUC)
	getH := tripGetHTTP.New(getUC)
	finishH := tripFinishHTTP.New(finishUC)

	serverImpl := appHTTP.NewServer(createH, getH, finishH, pool)

	// 6. Router & Middleware
	r := chi.NewRouter()
	appMiddleware.Chain(r, logger)
	api.HandlerFromMux(serverImpl, r)

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
