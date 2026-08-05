package kafka

import (
	"context"

	telemetrydomain "github.com/vishalss1/argus/core/internal/domain/telemetry"
)

// Deprecated: NoopTelemetryRepository is used when Core Service runs without Kafka producer configured.
// Telemetry is ingested directly by Telemetry Service via MQTT -> Kafka.
type NoopTelemetryRepository struct{}

func (NoopTelemetryRepository) Create(ctx context.Context, event telemetrydomain.Telemetry) (*telemetrydomain.Telemetry, error) {
	return &event, nil
}
