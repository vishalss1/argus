package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vishalss1/argus/core/internal/domain/workspace"
)

type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) Create(ctx context.Context, w workspace.Workspace) (*workspace.Workspace, error) {
	query := `
		INSERT INTO workspaces (id, name, description, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, created_at
	`

	var created workspace.Workspace
	err := r.db.QueryRowContext(
		ctx, query,
		w.ID, w.Name, w.Description, w.CreatedAt,
	).Scan(&created.ID, &created.Name, &created.Description, &created.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	return &created, nil
}

func (r *WorkspaceRepository) Get(ctx context.Context, id string) (*workspace.Workspace, error) {
	query := `SELECT id, name, description, created_at,
		(SELECT COUNT(*) FROM devices d WHERE d.workspace_id = workspaces.id) as device_count
		FROM workspaces WHERE id = $1`
	var w workspace.Workspace
	err := r.db.QueryRowContext(ctx, query, id).Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.DeviceCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return &w, nil
}

func (r *WorkspaceRepository) List(ctx context.Context) ([]workspace.Workspace, error) {
	query := `SELECT w.id, w.name, w.description, w.created_at,
		(SELECT COUNT(*) FROM devices d WHERE d.workspace_id = w.id) as device_count
		FROM workspaces w ORDER BY w.created_at DESC
		LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	defer rows.Close()

	var workspaces []workspace.Workspace
	for rows.Next() {
		var w workspace.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt, &w.DeviceCount); err != nil {
			return nil, fmt.Errorf("scan workspace: %w", err)
		}
		workspaces = append(workspaces, w)
	}
	return workspaces, nil
}

func (r *WorkspaceRepository) Update(ctx context.Context, id string, name string, description string) (*workspace.Workspace, error) {
	query := `
		UPDATE workspaces SET name = $1, description = $2
		WHERE id = $3
		RETURNING id, name, description, created_at
	`
	var w workspace.Workspace
	err := r.db.QueryRowContext(ctx, query, name, description, id).Scan(&w.ID, &w.Name, &w.Description, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}
	return &w, nil
}

func (r *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM workspaces WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}
	return nil
}

func (r *WorkspaceRepository) AssignDevice(ctx context.Context, workspaceID string, deviceID string) error {
	query := `UPDATE devices SET workspace_id = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, workspaceID, deviceID)
	if err != nil {
		return fmt.Errorf("assign device to workspace: %w", err)
	}
	return nil
}

func (r *WorkspaceRepository) UnassignDevice(ctx context.Context, workspaceID string, deviceID string) error {
	query := `UPDATE devices SET workspace_id = NULL WHERE id = $1 AND workspace_id = $2`
	_, err := r.db.ExecContext(ctx, query, deviceID, workspaceID)
	if err != nil {
		return fmt.Errorf("unassign device from workspace: %w", err)
	}
	return nil
}

func (r *WorkspaceRepository) ListDevices(ctx context.Context, workspaceID string) ([]workspace.DeviceSummary, error) {
	query := `SELECT id, name, type, status, firmware_version, last_seen FROM devices WHERE workspace_id = $1 ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace devices: %w", err)
	}
	defer rows.Close()

	var devices []workspace.DeviceSummary
	for rows.Next() {
		var d workspace.DeviceSummary
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &d.Status, &d.FirmwareVersion, &d.LastSeen); err != nil {
			return nil, fmt.Errorf("scan workspace device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}


