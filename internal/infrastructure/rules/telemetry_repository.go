package rules

import (
	"context"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/rule"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type TelemetryRepository struct {
	next    telemetry.Repository
	service *rule.Service
}

func NewTelemetryRepository(next telemetry.Repository, service *rule.Service) *TelemetryRepository {
	return &TelemetryRepository{
		next:    next,
		service: service,
	}
}

func (r *TelemetryRepository) Create(ctx context.Context, event telemetry.Telemetry) (*telemetry.Telemetry, error) {
	created, err := r.next.Create(ctx, event)
	if err != nil {
		return nil, err
	}

	if _, err := r.service.EvaluateTelemetry(ctx, *created); err != nil {
		return nil, fmt.Errorf("evaluate telemetry rules: %w", err)
	}

	return created, nil
}
