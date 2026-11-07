package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/505Eclypsia/template/internal/config"
	"github.com/505Eclypsia/template/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	c, err := container.New(context.Background(), cfg)
	if err != nil {
		log.Fatalf("init container: %v", err)
	}

	go func() {
		c.Logger.Info("server starting", "addr", cfg.HTTPAddr)
		if err := c.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			c.Logger.Error("server error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := c.Shutdown(shutdownCtx); err != nil {
		c.Logger.Error("shutdown error", "error", err)
	}
	c.Logger.Info("server stopped gracefully")
}
