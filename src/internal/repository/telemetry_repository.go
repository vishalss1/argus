package repository

import (
	"context"

	"github.com/vishalss1/argus/src/internal/model"
)

type TelemetryRepository interface {
	Create(ctx context.Context, telemetry model.Telemetry) (*model.Telemetry, error)
}
