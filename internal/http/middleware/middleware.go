package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func Chain(r *chi.Mux, logger *slog.Logger) {
	tracer := otel.GetTracerProvider().Tracer("trip-service-http")
	meter := otel.GetMeterProvider().Meter("trip-service-http")

	reqDuration, _ := meter.Float64Histogram("http.server.request.duration")
	activeRequests, _ := meter.Int64UpDownCounter("http.server.active_requests")

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						slog.Any("error", rec),
						slog.String("stack", string(debug.Stack())),
					)
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"code":"internal_error","title":"Internal Server Error","status":500,"detail":"Internal server error","type":"https://tripgo.example/problems/internal_error"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			start := time.Now()
			activeRequests.Add(req.Context(), 1)
			defer activeRequests.Add(req.Context(), -1)

			routePattern := chi.RouteContext(req.Context()).RoutePattern()
			if routePattern == "" {
				routePattern = req.URL.Path
			}

			spanName := fmt.Sprintf("%s %s", req.Method, routePattern)
			ctx, span := tracer.Start(req.Context(), spanName,
				trace.WithAttributes(
					attribute.String("http.route", routePattern),
					attribute.String("http.method", req.Method),
				),
			)
			defer span.End()

			ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)
			next.ServeHTTP(ww, req.WithContext(ctx))

			duration := time.Since(start)

			reqDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
				attribute.String("http.request.method", req.Method),
				attribute.String("http.route", routePattern),
				attribute.Int("http.response.status_code", ww.Status()),
			))

			logLevel := slog.LevelInfo
			if ww.Status() >= 500 {
				logLevel = slog.LevelError
			}

			logger.Log(ctx, logLevel, "request handled",
				slog.String("service", "trip-service"),
				slog.String("method", req.Method),
				slog.String("route", routePattern),
				slog.Int("status", ww.Status()),
				slog.Int64("duration_ms", duration.Milliseconds()),
			)
		})
	})
}
