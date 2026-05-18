package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/vishalss1/argus/internal/domain/device"
	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type TelemetryRepository struct {
	db *sql.DB
}

func NewTelemetryRepository(db *sql.DB) *TelemetryRepository {
	return &TelemetryRepository{db: db}
}

func (r *TelemetryRepository) Create(ctx context.Context, entity telemetry.Telemetry) (*telemetry.Telemetry, error) {
	const query = `
		INSERT INTO telemetry (id, device_id, recorded_at, metrics)
		VALUES ($1::uuid, $2::uuid, $3, $4::jsonb)
		RETURNING id, device_id, recorded_at, metrics, created_at`

	created, err := scanTelemetry(r.db.QueryRowContext(
		ctx,
		query,
		entity.ID,
		entity.DeviceID,
		entity.RecordedAt,
		entity.Metrics,
	))
	if isForeignKeyViolation(err) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create telemetry: %w", err)
	}

	return created, nil
}

type telemetryScanner interface {
	Scan(dest ...any) error
}

func scanTelemetry(scanner telemetryScanner) (*telemetry.Telemetry, error) {
	var entity telemetry.Telemetry
	err := scanner.Scan(
		&entity.ID,
		&entity.DeviceID,
		&entity.RecordedAt,
		&entity.Metrics,
		&entity.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}
