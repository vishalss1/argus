package kafka

import (
	"context"
	"fmt"

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
	created, err := r.next.Create(ctx, event)
	if err != nil {
		return nil, err
	}

	if err := r.producer.PublishTelemetry(ctx, *created); err != nil {
		return nil, fmt.Errorf("forward telemetry event: %w", err)
	}

	return created, nil
}
