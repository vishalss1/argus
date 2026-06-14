package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type Telemetry struct {
	Tracer trace.Tracer
	Meter  metric.Meter
}

// Init creates a basic Telemetry instance initialized from the global OTEL providers
func Init(serviceName string) (*Telemetry, error) {
	// In a real production setup, we would configure the OTLP exporter here.
	// For now, we just grab the global tracer and meter providers.
	tracer := otel.Tracer(serviceName)
	meter := otel.Meter(serviceName)

	return &Telemetry{
		Tracer: tracer,
		Meter:  meter,
	}, nil
}
