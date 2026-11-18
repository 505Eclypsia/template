package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	dE "github.com/505Eclypsia/template/internal/http/domainerror"
)

const unmatchedRoute = "unmatched"

type panicKey struct{}

type panicInfo struct {
	value any
}

func Chain(r *chi.Mux, logger *slog.Logger) {
	tracer := otel.GetTracerProvider().Tracer("trip-service-http")
	meter := otel.GetMeterProvider().Meter("trip-service-http")

	reqDuration, _ := meter.Float64Histogram("http.server.request.duration")
	activeRequests, _ := meter.Int64UpDownCounter("http.server.active_requests")

	r.Use(observe(logger, tracer, reqDuration, activeRequests))
	r.Use(recoverer(logger))
}

func observe(
	logger *slog.Logger,
	tracer trace.Tracer,
	reqDuration metric.Float64Histogram,
	activeRequests metric.Int64UpDownCounter,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			ctx, span := tracer.Start(ctx, req.Method,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(attribute.String("http.request.method", req.Method)),
			)
			defer span.End()

			panicked := &panicInfo{}
			ctx = context.WithValue(ctx, panicKey{}, panicked)

			ww := chiMiddleware.NewWrapResponseWriter(w, req.ProtoMajor)
			start := time.Now()

			activeAttrs := metric.WithAttributes(attribute.String("http.request.method", req.Method))
			activeRequests.Add(ctx, 1, activeAttrs)
			defer activeRequests.Add(ctx, -1, activeAttrs)

			defer func() {
				routePattern := unmatchedRoute
				if rctx := chi.RouteContext(req.Context()); rctx != nil && rctx.RoutePattern() != "" {
					routePattern = rctx.RoutePattern()
				}

				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
					if panicked.value != nil {
						status = http.StatusInternalServerError
					}
				}

				span.SetName(fmt.Sprintf("%s %s", req.Method, routePattern))
				span.SetAttributes(
					attribute.String("http.route", routePattern),
					attribute.Int("http.response.status_code", status),
				)
				if panicked.value != nil {
					span.RecordError(fmt.Errorf("panic: %v", panicked.value))
				}
				failed := status >= http.StatusInternalServerError || panicked.value != nil
				if failed {
					span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
				}

				duration := time.Since(start)

				reqDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
					attribute.String("http.request.method", req.Method),
					attribute.String("http.route", routePattern),
					attribute.Int("http.response.status_code", status),
				))

				level := slog.LevelInfo
				if failed {
					level = slog.LevelError
				}

				attrs := []slog.Attr{
					slog.String("service", "trip-service"),
					slog.String("method", req.Method),
					slog.String("route", routePattern),
					slog.Int("status", status),
					slog.Int64("duration_ms", duration.Milliseconds()),
				}
				if panicked.value != nil {
					attrs = append(attrs, slog.Any("panic", panicked.value))
				}
				logger.LogAttrs(ctx, level, "request handled", attrs...)
			}()

			next.ServeHTTP(ww, req.WithContext(ctx))
		})
	}
}

func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				if rec == http.ErrAbortHandler {
					panic(rec)
				}

				if info, ok := r.Context().Value(panicKey{}).(*panicInfo); ok {
					info.value = rec
				}

				logger.ErrorContext(r.Context(), "panic recovered",
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)

				if ww, ok := w.(chiMiddleware.WrapResponseWriter); ok && ww.Status() != 0 {
					return
				}
				dE.WriteProblem(w, r, errors.New("panic recovered"))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
