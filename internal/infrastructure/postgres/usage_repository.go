package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/usage"
)

type UsageRepository struct {
	db *sql.DB
}

func NewUsageRepository(db *sql.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

func (r *UsageRepository) GetUsage(ctx context.Context, tenantID string, month string) (*usage.Usage, error) {
	query := `
		SELECT tenant_id, billing_month, devices_used, workspaces_used, sessions_run, messages_processed
		FROM tenant_usage
		WHERE tenant_id = $1 AND billing_month = $2
	`
	var u usage.Usage
	err := r.db.QueryRowContext(ctx, query, tenantID, month).Scan(
		&u.TenantID, &u.BillingMonth, &u.DevicesUsed, &u.WorkspacesUsed, &u.SessionsRun, &u.MessagesProcessed,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get usage: %w", err)
	}
	return &u, nil
}

func (r *UsageRepository) IncrementSessions(ctx context.Context, tenantID string, month string) error {
	query := `
		INSERT INTO tenant_usage (tenant_id, billing_month, sessions_run)
		VALUES ($1, $2, 1)
		ON CONFLICT (tenant_id, billing_month) DO UPDATE SET
			sessions_run = tenant_usage.sessions_run + 1
	`
	_, err := r.db.ExecContext(ctx, query, tenantID, month)
	if err != nil {
		return fmt.Errorf("increment sessions: %w", err)
	}
	return nil
}

func (r *UsageRepository) UpdateUsage(ctx context.Context, tenantID string, month string, devices int, workspaces int) error {
	query := `
		INSERT INTO tenant_usage (tenant_id, billing_month, devices_used, workspaces_used)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, billing_month) DO UPDATE SET
			devices_used = EXCLUDED.devices_used,
			workspaces_used = EXCLUDED.workspaces_used
	`
	_, err := r.db.ExecContext(ctx, query, tenantID, month, devices, workspaces)
	if err != nil {
		return fmt.Errorf("update usage: %w", err)
	}
	return nil
}
