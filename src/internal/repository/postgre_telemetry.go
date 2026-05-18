package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/vishalss1/argus/src/internal/model"
)

type PostgreTelemetryRepository struct {
	db *sql.DB
}

func NewPostgreTelemetryRepository(db *sql.DB) *PostgreTelemetryRepository {
	return &PostgreTelemetryRepository{db: db}
}

func (r *PostgreTelemetryRepository) Create(ctx context.Context, telemetry model.Telemetry) (*model.Telemetry, error) {
	const query = `
		INSERT INTO telemetry (id, device_id, recorded_at, metrics)
		VALUES ($1::uuid, $2::uuid, $3, $4::jsonb)
		RETURNING id, device_id, recorded_at, metrics, created_at`

	created, err := scanTelemetry(r.db.QueryRowContext(
		ctx,
		query,
		telemetry.ID,
		telemetry.DeviceID,
		telemetry.RecordedAt,
		telemetry.Metrics,
	))
	if isForeignKeyViolation(err) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("create telemetry: %w", err)
	}

	return created, nil
}

type telemetryScanner interface {
	Scan(dest ...any) error
}

func scanTelemetry(scanner telemetryScanner) (*model.Telemetry, error) {
	var telemetry model.Telemetry
	err := scanner.Scan(
		&telemetry.ID,
		&telemetry.DeviceID,
		&telemetry.RecordedAt,
		&telemetry.Metrics,
		&telemetry.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &telemetry, nil
}

func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23503"
}
