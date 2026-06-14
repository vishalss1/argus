package kafka

import (
	"context"

	telemetrydomain "github.com/vishalss1/argus/core/internal/domain/telemetry"
)

type NoopTelemetryRepository struct{}

func (NoopTelemetryRepository) Create(ctx context.Context, event telemetrydomain.Telemetry) (*telemetrydomain.Telemetry, error) {
	return &event, nil
}
