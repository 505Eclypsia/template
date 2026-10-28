package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/505Eclypsia/template/internal/config"
)

type ShutdownFunc func(context.Context) error

func Setup(ctx context.Context, cfg *config.Config) (*slog.Logger, ShutdownFunc, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("trip-service"),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create resource: %w", err)
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:22318"
	}

	otlptracehttp.WithEndpoint(endpoint)
	otlpmetrichttp.WithEndpoint(endpoint)

	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint("localhost:22318"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint("localhost:22318"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}

	handler := &traceLogHandler{
		Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}),
	}
	logger := slog.New(handler)

	shutdown := func(ctx context.Context) error {
		_ = tp.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return nil
	}

	return logger, shutdown, nil
}

type traceLogHandler struct {
	slog.Handler
}

func (h *traceLogHandler) Handle(ctx context.Context, r slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}
