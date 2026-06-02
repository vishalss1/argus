package kafka

import (
	"context"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type TelemetryRepository struct {
	next     telemetry.Repository
	producer *Producer
}

func NewTelemetryRepository(next telemetry.Repository, producer *Producer) *TelemetryRepository {
	return &TelemetryRepository{
		next:     next,
		producer: producer,
	}
}

func (r *TelemetryRepository) Create(ctx context.Context, event telemetry.Telemetry) (*telemetry.Telemetry, error) {
	// Phase 1 Refactor: Bypass PostgreSQL write for raw telemetry.
	// Raw telemetry is now handled exclusively by the streaming pipeline.
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	if err := r.producer.PublishTelemetry(ctx, event); err != nil {
		return nil, fmt.Errorf("forward telemetry event: %w", err)
	}

	return &event, nil
}
