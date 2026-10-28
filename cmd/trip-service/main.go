package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/505Eclypsia/template/api"
	"github.com/505Eclypsia/template/internal/config"
	"github.com/505Eclypsia/template/internal/db"
	"github.com/505Eclypsia/template/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	log.Printf("config loaded successfully (HTTP_ADDR=%s)", cfg.HTTPAddr)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseConnTimeout)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to initialize db pool: %v", err)
	}
	defer pool.Close()
	log.Println("database pool connected and pinged successfully")

	repo := db.NewTripRepo(pool)
	txManager := db.NewTxManager(pool)
	h := handler.New(repo, txManager, pool)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	api.HandlerFromMux(h, r)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server is running on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited properly, all connections closed")

}
