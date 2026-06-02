package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/telemetry"
)

type FleetRepository struct {
	db *sql.DB
}

func NewFleetRepository(db *sql.DB) *FleetRepository {
	return &FleetRepository{db: db}
}

func (r *FleetRepository) Create(ctx context.Context, s telemetry.FleetSummary) (*telemetry.FleetSummary, error) {
	query := `
		INSERT INTO fleet_summaries (
			id, active_devices, offline_devices, avg_health_score, avg_risk_score, high_risk_devices, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, active_devices, offline_devices, avg_health_score, avg_risk_score, high_risk_devices, metadata, created_at
	`

	var created telemetry.FleetSummary
	err := r.db.QueryRowContext(
		ctx, query,
		s.ID, s.ActiveDevices, s.OfflineDevices, s.AvgHealthScore, s.AvgRiskScore, s.HighRiskDevices, s.Metadata, s.CreatedAt,
	).Scan(
		&created.ID, &created.ActiveDevices, &created.OfflineDevices, &created.AvgHealthScore, &created.AvgRiskScore, &created.HighRiskDevices, &created.Metadata, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create fleet summary: %w", err)
	}

	return &created, nil
}

func (r *FleetRepository) GetLatest(ctx context.Context) (*telemetry.FleetSummary, error) {
	query := `
		SELECT id, active_devices, offline_devices, avg_health_score, avg_risk_score, high_risk_devices, metadata, created_at
		FROM fleet_summaries
		ORDER BY created_at DESC
		LIMIT 1
	`

	var s telemetry.FleetSummary
	err := r.db.QueryRowContext(ctx, query).Scan(
		&s.ID, &s.ActiveDevices, &s.OfflineDevices, &s.AvgHealthScore, &s.AvgRiskScore, &s.HighRiskDevices, &s.Metadata, &s.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest fleet summary: %w", err)
	}

	return &s, nil
}

func (r *FleetRepository) List(ctx context.Context, limit int) ([]telemetry.FleetSummary, error) {
	query := `
		SELECT id, active_devices, offline_devices, avg_health_score, avg_risk_score, high_risk_devices, metadata, created_at
		FROM fleet_summaries
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list fleet summaries: %w", err)
	}
	defer rows.Close()

	var summaries []telemetry.FleetSummary
	for rows.Next() {
		var s telemetry.FleetSummary
		if err := rows.Scan(
			&s.ID, &s.ActiveDevices, &s.OfflineDevices, &s.AvgHealthScore, &s.AvgRiskScore, &s.HighRiskDevices, &s.Metadata, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan fleet summary: %w", err)
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}
