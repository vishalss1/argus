package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/shared/common"
	ctxdomain "github.com/vishalss1/argus/telemetry/internal/domain/context"
)

type ContextRepository struct {
	db *sql.DB
}

func NewContextRepository(db *sql.DB) *ContextRepository {
	return &ContextRepository{db: db}
}

func (r *ContextRepository) Create(ctx context.Context, mem ctxdomain.OperationalMemory) (*ctxdomain.OperationalMemory, error) {
	wID, _ := common.GetWorkspaceID(ctx)
	if wID == "" && mem.WorkspaceID != "" {
		wID = mem.WorkspaceID
	}

	query := `
		INSERT INTO operational_memory (
			id, device_id, workspace_id, type, summary, data, timestamp, created_at
		) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8)
		RETURNING id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
	`

	var created ctxdomain.OperationalMemory
	var wIDStr string
	err := r.db.QueryRowContext(
		ctx, query,
		mem.ID, mem.DeviceID, wID, mem.Type, mem.Summary, mem.Data, mem.Timestamp, mem.CreatedAt,
	).Scan(
		&created.ID, &created.DeviceID, &wIDStr, &created.Type, &created.Summary, &created.Data, &created.Timestamp, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create operational memory: %w", err)
	}
	created.WorkspaceID = wIDStr

	return &created, nil
}

func (r *ContextRepository) GetByID(ctx context.Context, id string) (*ctxdomain.OperationalMemory, error) {
	var query string
	var row *sql.Row

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE id = $1 AND workspace_id = $2::uuid
		`
		row = r.db.QueryRowContext(ctx, query, id, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE id = $1
		`
		row = r.db.QueryRowContext(ctx, query, id)
	}

	var mem ctxdomain.OperationalMemory
	var wIDStr string
	err := row.Scan(
		&mem.ID, &mem.DeviceID, &wIDStr, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}
	mem.WorkspaceID = wIDStr

	return &mem, nil
}

func (r *ContextRepository) ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]ctxdomain.OperationalMemory, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE device_id = $1 AND workspace_id = $4::uuid
			ORDER BY timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE device_id = $1
			ORDER BY timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("list operational memory by device: %w", err)
	}
	defer rows.Close()

	var memories []ctxdomain.OperationalMemory
	for rows.Next() {
		var mem ctxdomain.OperationalMemory
		var wIDStr string
		if err := rows.Scan(
			&mem.ID, &mem.DeviceID, &wIDStr, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan operational memory: %w", err)
		}
		mem.WorkspaceID = wIDStr
		memories = append(memories, mem)
	}

	return memories, nil
}

func (r *ContextRepository) ListByType(ctx context.Context, memoryType ctxdomain.MemoryType, limit, offset int) ([]ctxdomain.OperationalMemory, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE type = $1 AND workspace_id = $4::uuid
			ORDER BY timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, memoryType, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE type = $1
			ORDER BY timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, memoryType, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("list operational memory by type: %w", err)
	}
	defer rows.Close()

	var memories []ctxdomain.OperationalMemory
	for rows.Next() {
		var mem ctxdomain.OperationalMemory
		var wIDStr string
		if err := rows.Scan(
			&mem.ID, &mem.DeviceID, &wIDStr, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan operational memory: %w", err)
		}
		mem.WorkspaceID = wIDStr
		memories = append(memories, mem)
	}

	return memories, nil
}

func (r *ContextRepository) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM operational_memory WHERE created_at < $1`
	res, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune operational memory: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func (r *ContextRepository) GetLatestByDevice(ctx context.Context, deviceID string, memoryType ctxdomain.MemoryType) (*ctxdomain.OperationalMemory, error) {
	var query string
	var row *sql.Row

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE device_id = $1 AND type = $2 AND workspace_id = $3::uuid
			ORDER BY timestamp DESC
			LIMIT 1
		`
		row = r.db.QueryRowContext(ctx, query, deviceID, memoryType, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE device_id = $1 AND type = $2
			ORDER BY timestamp DESC
			LIMIT 1
		`
		row = r.db.QueryRowContext(ctx, query, deviceID, memoryType)
	}

	var mem ctxdomain.OperationalMemory
	var wIDStr string
	err := row.Scan(
		&mem.ID, &mem.DeviceID, &wIDStr, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest operational memory: %w", err)
	}
	mem.WorkspaceID = wIDStr

	return &mem, nil
}
