package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/finding"
)

type FindingRepository struct {
	db *sql.DB
}

func NewFindingRepository(db *sql.DB) *FindingRepository {
	return &FindingRepository{db: db}
}

func (r *FindingRepository) Create(ctx context.Context, f finding.Finding) (*finding.Finding, error) {
	query := `
		INSERT INTO ai_findings (id, device_id, risk_score, health_score, severity, summary, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, device_id, risk_score, health_score, severity, summary, metadata, created_at
	`

	var created finding.Finding
	err := r.db.QueryRowContext(
		ctx, query,
		f.ID, f.DeviceID, f.RiskScore, f.HealthScore, f.Severity, f.Summary, f.Metadata, f.CreatedAt,
	).Scan(
		&created.ID, &created.DeviceID, &created.RiskScore, &created.HealthScore, &created.Severity, &created.Summary, &created.Metadata, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create ai finding: %w", err)
	}

	return &created, nil
}

func (r *FindingRepository) ListByDevice(ctx context.Context, deviceID string) ([]finding.Finding, error) {
	query := `
		SELECT id, device_id, risk_score, health_score, severity, summary, metadata, created_at
		FROM ai_findings
		WHERE device_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, fmt.Errorf("list ai findings by device: %w", err)
	}
	defer rows.Close()

	var findings []finding.Finding
	for rows.Next() {
		var f finding.Finding
		if err := rows.Scan(
			&f.ID, &f.DeviceID, &f.RiskScore, &f.HealthScore, &f.Severity, &f.Summary, &f.Metadata, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ai finding: %w", err)
		}
		findings = append(findings, f)
	}

	return findings, nil
}
