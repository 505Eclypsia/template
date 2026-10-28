package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type BusinessMetrics struct {
	tripsCreated   metric.Int64Counter
	tripsCompleted metric.Int64Counter
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

	Metrics = &BusinessMetrics{
		tripsCreated:   created,
		tripsCompleted: completed,
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
