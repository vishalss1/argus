package postgres

import (
	"context"
	"database/sql"
	"fmt"

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

func (r *ContextRepository) ListByDevice(ctx context.Context, deviceID string) ([]ctxdomain.OperationalMemory, error) {
	query := `
		SELECT id, device_id, type, summary, data, timestamp, created_at
		FROM operational_memory
		WHERE device_id = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
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

func (r *ContextRepository) ListByType(ctx context.Context, memoryType ctxdomain.MemoryType) ([]ctxdomain.OperationalMemory, error) {
	query := `
		SELECT id, device_id, type, summary, data, timestamp, created_at
		FROM operational_memory
		WHERE type = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query, memoryType)
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

func (r *ContextRepository) GetLatestByDevice(ctx context.Context, deviceID string, memoryType ctxdomain.MemoryType) (*ctxdomain.OperationalMemory, error) {
	query := `
		SELECT id, device_id, type, summary, data, timestamp, created_at
		FROM operational_memory
		WHERE device_id = $1 AND type = $2
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var mem ctxdomain.OperationalMemory
	err := r.db.QueryRowContext(ctx, query, deviceID, memoryType).Scan(
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
