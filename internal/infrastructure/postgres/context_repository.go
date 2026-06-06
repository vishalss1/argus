package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/auth"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
)

type ContextRepository struct {
	db *sql.DB
}

func NewContextRepository(db *sql.DB) *ContextRepository {
	return &ContextRepository{db: db}
}

func (r *ContextRepository) Create(ctx context.Context, mem ctxdomain.OperationalMemory) (*ctxdomain.OperationalMemory, error) {
	query := `
		INSERT INTO operational_memory (
			id, device_id, type, summary, data, timestamp, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, device_id, type, summary, data, timestamp, created_at
	`

	var created ctxdomain.OperationalMemory
	err := r.db.QueryRowContext(
		ctx, query,
		mem.ID, mem.DeviceID, mem.Type, mem.Summary, mem.Data, mem.Timestamp, mem.CreatedAt,
	).Scan(
		&created.ID, &created.DeviceID, &created.Type, &created.Summary, &created.Data, &created.Timestamp, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create operational memory: %w", err)
	}

	return &created, nil
}

func (r *ContextRepository) GetByID(ctx context.Context, id string) (*ctxdomain.OperationalMemory, error) {
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT om.id, om.device_id, om.type, om.summary, om.data, om.timestamp, om.created_at
			FROM operational_memory om
			JOIN devices d ON om.device_id = d.id
			WHERE om.id = $1 AND d.workspace_id = $2::uuid
		`
		row = r.db.QueryRowContext(ctx, query, id, wID)
	} else {
		query = `
			SELECT id, device_id, type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE id = $1
		`
		row = r.db.QueryRowContext(ctx, query, id)
	}

	var mem ctxdomain.OperationalMemory
	err := row.Scan(
		&mem.ID, &mem.DeviceID, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("memory not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}

	return &mem, nil
}

func (r *ContextRepository) ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]ctxdomain.OperationalMemory, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT om.id, om.device_id, om.type, om.summary, om.data, om.timestamp, om.created_at
			FROM operational_memory om
			JOIN devices d ON om.device_id = d.id
			WHERE om.device_id = $1 AND d.workspace_id = $4::uuid
			ORDER BY om.timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, type, summary, data, timestamp, created_at
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
		if err := rows.Scan(
			&mem.ID, &mem.DeviceID, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan operational memory: %w", err)
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

func (r *ContextRepository) ListByType(ctx context.Context, memoryType ctxdomain.MemoryType, limit, offset int) ([]ctxdomain.OperationalMemory, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT om.id, om.device_id, om.type, om.summary, om.data, om.timestamp, om.created_at
			FROM operational_memory om
			JOIN devices d ON om.device_id = d.id
			WHERE om.type = $1 AND d.workspace_id = $4::uuid
			ORDER BY om.timestamp DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, memoryType, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, type, summary, data, timestamp, created_at
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
		if err := rows.Scan(
			&mem.ID, &mem.DeviceID, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan operational memory: %w", err)
		}
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

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT om.id, om.device_id, om.type, om.summary, om.data, om.timestamp, om.created_at
			FROM operational_memory om
			JOIN devices d ON om.device_id = d.id
			WHERE om.device_id = $1 AND om.type = $2 AND d.workspace_id = $3::uuid
			ORDER BY om.timestamp DESC
			LIMIT 1
		`
		row = r.db.QueryRowContext(ctx, query, deviceID, memoryType, wID)
	} else {
		query = `
			SELECT id, device_id, type, summary, data, timestamp, created_at
			FROM operational_memory
			WHERE device_id = $1 AND type = $2
			ORDER BY timestamp DESC
			LIMIT 1
		`
		row = r.db.QueryRowContext(ctx, query, deviceID, memoryType)
	}

	var mem ctxdomain.OperationalMemory
	err := row.Scan(
		&mem.ID, &mem.DeviceID, &mem.Type, &mem.Summary, &mem.Data, &mem.Timestamp, &mem.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest operational memory: %w", err)
	}

	return &mem, nil
}
