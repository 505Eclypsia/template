package telemetry

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type BusinessMetrics struct {
	tripsCreated   metric.Int64Counter
	tripsCompleted metric.Int64Counter

	// ЛР3
	positionsSaved        metric.Int64Counter
	staleChecks           metric.Int64Counter
	positionRequests      metric.Int64Counter
	positionQueueSize     metric.Int64UpDownCounter
	positionWorkersActive metric.Int64UpDownCounter
	positionReqDuration   metric.Float64Histogram
}

var Metrics *BusinessMetrics

func InitBusinessMetrics() error {
	meter := otel.GetMeterProvider().Meter("trip-service")

	created, err := meter.Int64Counter("tripgo.trips.created",
		metric.WithDescription("Total number of created trips"),
	)
	if err != nil {
		return err
	}

	completed, err := meter.Int64Counter("tripgo.trips.completed",
		metric.WithDescription("Total number of completed trips"),
	)
	if err != nil {
		return err
	}

	saved, err := meter.Int64Counter("tripgo.positions.saved",
		metric.WithDescription("Total number of saved trip positions"),
	)
	if err != nil {
		return err
	}

	staleChecks, err := meter.Int64Counter("tripgo.stale_checks",
		metric.WithDescription("Results of putting stale trips into the queue (queued, duplicate, dropped)"),
	)
	if err != nil {
		return err
	}

	requests, err := meter.Int64Counter("tripgo.position_requests",
		metric.WithDescription("Results of position requests to Push Service (success, error, cancelled)"),
	)
	if err != nil {
		return err
	}

	queueSize, err := meter.Int64UpDownCounter("tripgo.position_requests.queue.size",
		metric.WithDescription("Current number of trips waiting in the position request queue"),
	)
	if err != nil {
		return err
	}

	workersActive, err := meter.Int64UpDownCounter("tripgo.position_requests.workers.active",
		metric.WithDescription("Current number of workers busy with a position request"),
	)
	if err != nil {
		return err
	}

	duration, err := meter.Float64Histogram("tripgo.position_requests.duration",
		metric.WithDescription("Duration of position requests to Push Service"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	Metrics = &BusinessMetrics{
		tripsCreated:          created,
		tripsCompleted:        completed,
		positionsSaved:        saved,
		staleChecks:           staleChecks,
		positionRequests:      requests,
		positionQueueSize:     queueSize,
		positionWorkersActive: workersActive,
		positionReqDuration:   duration,
	}
	return nil
}

func (m *BusinessMetrics) IncTripsCreated(ctx context.Context) {
	if m != nil && m.tripsCreated != nil {
		m.tripsCreated.Add(ctx, 1)
	}
}

func (m *BusinessMetrics) IncTripsCompleted(ctx context.Context) {
	if m != nil && m.tripsCompleted != nil {
		m.tripsCompleted.Add(ctx, 1)
	}
}

func (m *BusinessMetrics) IncPositionsSaved(ctx context.Context) {
	if m != nil && m.positionsSaved != nil {
		m.positionsSaved.Add(ctx, 1)
	}
}

func (m *BusinessMetrics) IncStaleCheck(ctx context.Context, result string) {
	if m != nil && m.staleChecks != nil {
		m.staleChecks.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *BusinessMetrics) IncPositionRequest(ctx context.Context, result string) {
	if m != nil && m.positionRequests != nil {
		m.positionRequests.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *BusinessMetrics) RecordPositionRequestDuration(ctx context.Context, d time.Duration, result string) {
	if m != nil && m.positionReqDuration != nil {
		m.positionReqDuration.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("result", result)))
	}
}

func (m *BusinessMetrics) AddQueueSize(ctx context.Context, delta int64) {
	if m != nil && m.positionQueueSize != nil {
		m.positionQueueSize.Add(ctx, delta)
	}
}

func (m *BusinessMetrics) AddActiveWorkers(ctx context.Context, delta int64) {
	if m != nil && m.positionWorkersActive != nil {
		m.positionWorkersActive.Add(ctx, delta)
	}
}
