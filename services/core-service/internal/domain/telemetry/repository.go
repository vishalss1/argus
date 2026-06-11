package telemetry

import "context"

type Repository interface {
	Create(ctx context.Context, telemetry Telemetry) (*Telemetry, error)
}

