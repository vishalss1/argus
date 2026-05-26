package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/vishalss1/argus/internal/domain/policy"
)

type PolicyRepository struct {
	db *sql.DB
}

func NewPolicyRepository(db *sql.DB) *PolicyRepository {
	return &PolicyRepository{db: db}
}

func (r *PolicyRepository) CreatePolicy(ctx context.Context, p policy.Policy) (*policy.Policy, error) {
	query := `
		INSERT INTO policies (id, action, allowed_devices, requires_approval, max_per_day, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, action, allowed_devices, requires_approval, max_per_day, created_at, updated_at
	`
	now := time.Now()
	var created policy.Policy
	err := r.db.QueryRowContext(ctx, query, p.ID, p.Action, pq.Array(p.AllowedDevices), p.RequiresApproval, p.MaxPerDay, now, now).Scan(
		&created.ID, &created.Action, pq.Array(&created.AllowedDevices), &created.RequiresApproval, &created.MaxPerDay, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create policy: %w", err)
	}
	return &created, nil
}

func (r *PolicyRepository) GetPolicyByAction(ctx context.Context, action policy.ActionType) (*policy.Policy, error) {
	query := `
		SELECT id, action, allowed_devices, requires_approval, max_per_day, created_at, updated_at
		FROM policies
		WHERE action = $1
	`
	var p policy.Policy
	err := r.db.QueryRowContext(ctx, query, action).Scan(
		&p.ID, &p.Action, pq.Array(&p.AllowedDevices), &p.RequiresApproval, &p.MaxPerDay, &p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		// Return default policy if not found
		return &policy.Policy{
			Action:           action,
			RequiresApproval: true,
			MaxPerDay:        10,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}
	return &p, nil
}

func (r *PolicyRepository) ListPolicies(ctx context.Context) ([]policy.Policy, error) {
	query := `SELECT id, action, allowed_devices, requires_approval, max_per_day, created_at, updated_at FROM policies`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []policy.Policy
	for rows.Next() {
		var p policy.Policy
		if err := rows.Scan(&p.ID, &p.Action, pq.Array(&p.AllowedDevices), &p.RequiresApproval, &p.MaxPerDay, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, nil
}

func (r *PolicyRepository) UpdatePolicy(ctx context.Context, p policy.Policy) (*policy.Policy, error) {
	query := `
		UPDATE policies
		SET allowed_devices = $2, requires_approval = $3, max_per_day = $4, updated_at = $5
		WHERE id = $1
		RETURNING id, action, allowed_devices, requires_approval, max_per_day, created_at, updated_at
	`
	var updated policy.Policy
	err := r.db.QueryRowContext(ctx, query, p.ID, pq.Array(p.AllowedDevices), p.RequiresApproval, p.MaxPerDay, time.Now()).Scan(
		&updated.ID, &updated.Action, pq.Array(&updated.AllowedDevices), &updated.RequiresApproval, &updated.MaxPerDay, &updated.CreatedAt, &updated.UpdatedAt,
	)
	return &updated, err
}

func (r *PolicyRepository) DeletePolicy(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM policies WHERE id = $1", id)
	return err
}

func (r *PolicyRepository) CreateExecutionRecord(ctx context.Context, record policy.ExecutionRecord) (*policy.ExecutionRecord, error) {
	query := `
		INSERT INTO action_execution_records (id, action, device_id, incident_id, status, suggested_by, approved_by, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, action, device_id, incident_id, status, suggested_by, approved_by, metadata, created_at
	`
	var created policy.ExecutionRecord
	err := r.db.QueryRowContext(ctx, query, record.ID, record.Action, record.DeviceID, record.IncidentID, record.Status, record.SuggestedBy, record.ApprovedBy, record.Metadata, record.CreatedAt).Scan(
		&created.ID, &created.Action, &created.DeviceID, &created.IncidentID, &created.Status, &created.SuggestedBy, &created.ApprovedBy, &created.Metadata, &created.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create execution record: %w", err)
	}
	return &created, nil
}

func (r *PolicyRepository) GetExecutionRecord(ctx context.Context, id string) (*policy.ExecutionRecord, error) {
	query := `
		SELECT id, action, device_id, incident_id, status, suggested_by, approved_by, metadata, created_at
		FROM action_execution_records
		WHERE id = $1
	`
	var record policy.ExecutionRecord
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&record.ID, &record.Action, &record.DeviceID, &record.IncidentID, &record.Status, &record.SuggestedBy, &record.ApprovedBy, &record.Metadata, &record.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *PolicyRepository) ListExecutionRecords(ctx context.Context) ([]policy.ExecutionRecord, error) {
	query := `SELECT id, action, device_id, incident_id, status, suggested_by, approved_by, metadata, created_at FROM action_execution_records ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []policy.ExecutionRecord
	for rows.Next() {
		var record policy.ExecutionRecord
		if err := rows.Scan(&record.ID, &record.Action, &record.DeviceID, &record.IncidentID, &record.Status, &record.SuggestedBy, &record.ApprovedBy, &record.Metadata, &record.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (r *PolicyRepository) UpdateExecutionStatus(ctx context.Context, id string, status string, approvedBy *string) error {
	query := `UPDATE action_execution_records SET status = $2, approved_by = $3 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id, status, approvedBy)
	return err
}
