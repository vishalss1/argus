package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/domain/fleet"
	"github.com/vishalss1/argus/shared/common"
)

type FleetRepository struct {
	db *sql.DB
}

func NewFleetRepository(db *sql.DB) *FleetRepository {
	return &FleetRepository{db: db}
}

func (r *FleetRepository) Create(ctx context.Context, f fleet.Fleet) (*fleet.Fleet, error) {
	const query = `
		INSERT INTO fleets (
			workspace_id, name, node_role, hardware_type, node_prefix,
			firmware_version, firmware_template, node_count
		) VALUES (
			$1::uuid, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id, workspace_id, name, node_role, hardware_type, node_prefix,
		firmware_version, firmware_template, node_count, created_at, updated_at
	`

	var created fleet.Fleet
	err := r.db.QueryRowContext(
		ctx, query,
		f.WorkspaceID, f.Name, f.NodeRole, f.HardwareType, f.NodePrefix,
		f.FirmwareVersion, f.FirmwareTemplate, f.NodeCount,
	).Scan(
		&created.ID, &created.WorkspaceID, &created.Name, &created.NodeRole,
		&created.HardwareType, &created.NodePrefix, &created.FirmwareVersion,
		&created.FirmwareTemplate, &created.NodeCount, &created.CreatedAt, &created.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("insert fleet: %w", err)
	}

	return &created, nil
}

func (r *FleetRepository) List(ctx context.Context) ([]fleet.FleetWithStats, error) {
	wID, ok := common.GetWorkspaceID(ctx)
	if !ok {
		return nil, errors.New("workspace ID required")
	}

	const query = `
		SELECT 
			f.id, f.workspace_id, f.name, f.node_role, f.hardware_type, f.node_prefix,
			f.firmware_version, f.firmware_template, f.node_count, f.created_at, f.updated_at,
			COUNT(d.id) as total_nodes,
			COUNT(d.id) FILTER (WHERE d.status = 'online') as online_nodes,
			COUNT(d.id) FILTER (WHERE d.status = 'offline') as offline_nodes
		FROM fleets f
		LEFT JOIN devices d ON d.fleet_id = f.id
		WHERE f.workspace_id = $1::uuid
		GROUP BY f.id
		ORDER BY f.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, wID)
	if err != nil {
		return nil, fmt.Errorf("list fleets query: %w", err)
	}
	defer rows.Close()

	var fleets []fleet.FleetWithStats
	for rows.Next() {
		var fs fleet.FleetWithStats
		err := rows.Scan(
			&fs.ID, &fs.WorkspaceID, &fs.Name, &fs.NodeRole, &fs.HardwareType, &fs.NodePrefix,
			&fs.FirmwareVersion, &fs.FirmwareTemplate, &fs.NodeCount, &fs.CreatedAt, &fs.UpdatedAt,
			&fs.TotalNodes, &fs.OnlineNodes, &fs.OfflineNodes,
		)
		if err != nil {
			return nil, fmt.Errorf("list fleets scan: %w", err)
		}
		fleets = append(fleets, fs)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list fleets rows: %w", err)
	}

	return fleets, nil
}

func (r *FleetRepository) GetByID(ctx context.Context, id string) (*fleet.Fleet, error) {
	wID, ok := common.GetWorkspaceID(ctx)
	if !ok {
		return nil, errors.New("workspace ID required")
	}

	const query = `
		SELECT 
			id, workspace_id, name, node_role, hardware_type, node_prefix,
			firmware_version, firmware_template, node_count, created_at, updated_at
		FROM fleets
		WHERE id = $1::uuid AND workspace_id = $2::uuid
	`

	var f fleet.Fleet
	err := r.db.QueryRowContext(ctx, query, id, wID).Scan(
		&f.ID, &f.WorkspaceID, &f.Name, &f.NodeRole, &f.HardwareType, &f.NodePrefix,
		&f.FirmwareVersion, &f.FirmwareTemplate, &f.NodeCount, &f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fleet.ErrFleetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get fleet query: %w", err)
	}

	return &f, nil
}

func (r *FleetRepository) GetWithDevices(ctx context.Context, id string) (*fleet.FleetWithStats, error) {
	wID, ok := common.GetWorkspaceID(ctx)
	if !ok {
		return nil, errors.New("workspace ID required")
	}

	const fleetQuery = `
		SELECT 
			f.id, f.workspace_id, f.name, f.node_role, f.hardware_type, f.node_prefix,
			f.firmware_version, f.firmware_template, f.node_count, f.created_at, f.updated_at,
			COUNT(d.id) as total_nodes,
			COUNT(d.id) FILTER (WHERE d.status = 'online') as online_nodes,
			COUNT(d.id) FILTER (WHERE d.status = 'offline') as offline_nodes
		FROM fleets f
		LEFT JOIN devices d ON d.fleet_id = f.id
		WHERE f.id = $1::uuid AND f.workspace_id = $2::uuid
		GROUP BY f.id
	`

	var fs fleet.FleetWithStats
	err := r.db.QueryRowContext(ctx, fleetQuery, id, wID).Scan(
		&fs.ID, &fs.WorkspaceID, &fs.Name, &fs.NodeRole, &fs.HardwareType, &fs.NodePrefix,
		&fs.FirmwareVersion, &fs.FirmwareTemplate, &fs.NodeCount, &fs.CreatedAt, &fs.UpdatedAt,
		&fs.TotalNodes, &fs.OnlineNodes, &fs.OfflineNodes,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fleet.ErrFleetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get fleet with stats query: %w", err)
	}

	const devicesQuery = `
		SELECT id, name, type, firmware_version, status, metadata, last_seen, workspace_id, fleet_id, created_at, updated_at, api_key_hash, api_key_prefix
		FROM devices
		WHERE fleet_id = $1::uuid
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, devicesQuery, id)
	if err != nil {
		return nil, fmt.Errorf("get fleet devices query: %w", err)
	}
	defer rows.Close()

	fs.Devices = make([]device.Device, 0)
	for rows.Next() {
		var dev device.Device
		err := rows.Scan(
			&dev.ID, &dev.Name, &dev.Type, &dev.FirmwareVersion, &dev.Status, &dev.Metadata,
			&dev.LastSeen, &dev.WorkspaceID, &dev.FleetID, &dev.CreatedAt, &dev.UpdatedAt,
			&dev.APIKeyHash, &dev.APIKeyPrefix,
		)
		if err != nil {
			return nil, fmt.Errorf("get fleet devices scan: %w", err)
		}
		fs.Devices = append(fs.Devices, dev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get fleet devices rows: %w", err)
	}

	return &fs, nil
}

func (r *FleetRepository) Delete(ctx context.Context, id string) error {
	wID, ok := common.GetWorkspaceID(ctx)
	if !ok {
		return errors.New("workspace ID required")
	}

	result, err := r.db.ExecContext(ctx, "DELETE FROM fleets WHERE id = $1::uuid AND workspace_id = $2::uuid", id, wID)
	if err != nil {
		return fmt.Errorf("delete fleet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete fleet rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fleet.ErrFleetNotFound
	}

	return nil
}
