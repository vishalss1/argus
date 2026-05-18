package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vishalss1/argus/src/internal/model"
)

type PostgreDeviceRepository struct {
	db *sql.DB
}

func NewPostgreDeviceRepository(db *sql.DB) *PostgreDeviceRepository {
	return &PostgreDeviceRepository{db: db}
}

func (r *PostgreDeviceRepository) Create(ctx context.Context, device model.Device) (*model.Device, error) {
	metadata := device.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	const query = `
		INSERT INTO devices (id, name, type, firmware_version, status, metadata)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	created, err := scanDevice(r.db.QueryRowContext(
		ctx,
		query,
		device.ID,
		device.Name,
		device.Type,
		device.FirmwareVersion,
		device.Status,
		metadata,
	))
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}

	return created, nil
}

func (r *PostgreDeviceRepository) List(ctx context.Context) ([]model.Device, error) {
	const query = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at
		FROM devices
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := make([]model.Device, 0)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *device)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list devices rows: %w", err)
	}

	return devices, nil
}

func (r *PostgreDeviceRepository) GetByID(ctx context.Context, id string) (*model.Device, error) {
	const query = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at
		FROM devices
		WHERE id = $1::uuid`

	device, err := scanDevice(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	return device, nil
}

func (r *PostgreDeviceRepository) Update(ctx context.Context, id string, req model.UpdateDeviceRequest) (*model.Device, error) {
	current, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		current.Name = *req.Name
	}
	if req.Type != nil {
		current.Type = *req.Type
	}
	if req.FirmwareVersion != nil {
		current.FirmwareVersion = *req.FirmwareVersion
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	if req.Metadata != nil {
		current.Metadata = *req.Metadata
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
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}

	return updated, nil
}

func (r *PostgreDeviceRepository) UpdateHeartbeat(ctx context.Context, id string, status string) (*model.Device, error) {
	const query = `
		UPDATE devices
		SET status = $2,
			last_seen = NOW(),
			updated_at = NOW()
		WHERE id = $1::uuid
		RETURNING id, name, type, firmware_version, status, metadata, last_seen, created_at, updated_at`

	device, err := scanDevice(r.db.QueryRowContext(ctx, query, id, status))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDeviceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update device heartbeat: %w", err)
	}

	return device, nil
}

func (r *PostgreDeviceRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM devices WHERE id = $1::uuid", id)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete device rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return ErrDeviceNotFound
	}

	return nil
}

type deviceScanner interface {
	Scan(dest ...any) error
}

func scanDevice(scanner deviceScanner) (*model.Device, error) {
	var device model.Device
	err := scanner.Scan(
		&device.ID,
		&device.Name,
		&device.Type,
		&device.FirmwareVersion,
		&device.Status,
		&device.Metadata,
		&device.LastSeen,
		&device.CreatedAt,
		&device.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &device, nil
}
