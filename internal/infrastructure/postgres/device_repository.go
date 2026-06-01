package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/device"
)

type DeviceRepository struct {
	db *sql.DB
}

func NewDeviceRepository(db *sql.DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) Create(ctx context.Context, entity device.Device) (*device.Device, error) {
	metadata := entity.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	const query = `
		INSERT INTO devices (id, name, type, firmware_version, status, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			type = EXCLUDED.type,
			firmware_version = EXCLUDED.firmware_version,
			status = EXCLUDED.status,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	created, err := scanDevice(r.db.QueryRowContext(
		ctx,
		query,
		entity.ID,
		entity.Name,
		entity.Type,
		entity.FirmwareVersion,
		entity.Status,
		metadata,
	))
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}

	return created, nil
}

func (r *DeviceRepository) List(ctx context.Context) ([]device.Device, error) {
	const query = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at
		FROM devices
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := make([]device.Device, 0)
	for rows.Next() {
		entity, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list devices rows: %w", err)
	}

	return devices, nil
}

func (r *DeviceRepository) GetByID(ctx context.Context, id string) (*device.Device, error) {
	const query = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at
		FROM devices
		WHERE id = $1::uuid`

	entity, err := scanDevice(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	return entity, nil
}

func (r *DeviceRepository) GetByHardwareID(ctx context.Context, hardwareID string) (*device.Device, error) {
	const query = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at
		FROM devices
		WHERE metadata->>'hardware_id' = $1
		ORDER BY created_at ASC
		LIMIT 1`

	entity, err := scanDevice(r.db.QueryRowContext(ctx, query, hardwareID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device by hardware id: %w", err)
	}

	return entity, nil
}

func (r *DeviceRepository) Update(ctx context.Context, id string, input device.UpdateInput) (*device.Device, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.Type != nil {
		current.Type = *input.Type
	}
	if input.FirmwareVersion != nil {
		current.FirmwareVersion = *input.FirmwareVersion
	}
	if input.Status != nil {
		current.Status = *input.Status
	}
	if input.Metadata != nil {
		current.Metadata = *input.Metadata
	}

	const query = `
		UPDATE devices
		SET name = $2,
			type = $3,
			firmware_version = $4,
			status = $5,
			metadata = $6::jsonb,
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	updated, err := scanDevice(r.db.QueryRowContext(
		ctx,
		query,
		current.ID,
		current.Name,
		current.Type,
		current.FirmwareVersion,
		current.Status,
		current.Metadata,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}

	return updated, nil
}

func (r *DeviceRepository) UpdateHeartbeat(ctx context.Context, id string, status string) (*device.Device, error) {
	const query = `
		UPDATE devices
		SET status = $2,
			last_seen = NOW(),
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	entity, err := scanDevice(r.db.QueryRowContext(ctx, query, id, status))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update device heartbeat: %w", err)
	}

	return entity, nil
}

func (r *DeviceRepository) UpdatePresence(ctx context.Context, id string, status string, timestamp time.Time) (*device.Device, error) {
	const query = `
		UPDATE devices
		SET status = $2,
			last_seen = $3,
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	entity, err := scanDevice(r.db.QueryRowContext(ctx, query, id, status, timestamp.UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, device.ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update device presence: %w", err)
	}

	return entity, nil
}

func (r *DeviceRepository) MarkStaleOffline(ctx context.Context, timeout time.Duration) ([]device.Device, error) {
	const query = `
		UPDATE devices
		SET status = 'offline',
			updated_at = NOW()
		WHERE status <> 'offline'
			AND last_seen IS NOT NULL
			AND last_seen < NOW() - ($1::bigint * INTERVAL '1 second')
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	rows, err := r.db.QueryContext(ctx, query, int64(timeout.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("mark stale devices offline: %w", err)
	}
	defer rows.Close()

	devices := make([]device.Device, 0)
	for rows.Next() {
		entity, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mark stale devices offline rows: %w", err)
	}

	return devices, nil
}

func (r *DeviceRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM devices WHERE id = $1::uuid", id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete device rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return device.ErrDeviceNotFound
	}

	return nil
}

type deviceScanner interface {
	Scan(dest ...any) error
}

func scanDevice(scanner deviceScanner) (*device.Device, error) {
	var entity device.Device
	err := scanner.Scan(
		&entity.ID,
		&entity.Name,
		&entity.Type,
		&entity.FirmwareVersion,
		&entity.Status,
		&entity.Metadata,
		&entity.LastSeen,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}
