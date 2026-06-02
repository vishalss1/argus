package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type TelemetryRepository struct {
	db *sql.DB
}

func NewTelemetryRepository(db *sql.DB) *TelemetryRepository {
	return &TelemetryRepository{db: db}
}

func (r *TelemetryRepository) Create(ctx context.Context, entity telemetry.Telemetry) (*telemetry.Telemetry, error) {
	// Phase 1 Refactor: Stop writing raw telemetry to PostgreSQL.
	// We return the entity as if it were created, ensuring the pipeline continues.
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now().UTC()
	}
	return &entity, nil
}
