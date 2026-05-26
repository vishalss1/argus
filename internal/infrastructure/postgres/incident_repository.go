package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/vishalss1/argus/internal/domain/incident"
)

type IncidentRepository struct {
	db *sql.DB
}

func NewIncidentRepository(db *sql.DB) *IncidentRepository {
	return &IncidentRepository{db: db}
}

func (r *IncidentRepository) Create(ctx context.Context, inc incident.Incident) (*incident.Incident, error) {
	query := `
		INSERT INTO incidents (
			id, title, summary, severity, status, device_ids, event_ids, started_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, title, summary, severity, status, device_ids, event_ids, started_at, resolved_at, created_at, updated_at
	`

	var created incident.Incident
	err := r.db.QueryRowContext(
		ctx, query,
		inc.ID, inc.Title, inc.Summary, inc.Severity, inc.Status, pq.Array(inc.DeviceIDs), pq.Array(inc.EventIDs), inc.StartedAt, inc.CreatedAt, inc.UpdatedAt,
	).Scan(
		&created.ID, &created.Title, &created.Summary, &created.Severity, &created.Status, pq.Array(&created.DeviceIDs), pq.Array(&created.EventIDs), &created.StartedAt, &created.ResolvedAt, &created.CreatedAt, &created.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create incident: %w", err)
	}

	return &created, nil
}

func (r *IncidentRepository) GetByID(ctx context.Context, id string) (*incident.Incident, error) {
	query := `
		SELECT id, title, summary, severity, status, device_ids, event_ids, started_at, resolved_at, created_at, updated_at
		FROM incidents
		WHERE id = $1
	`

	var inc incident.Incident
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&inc.ID, &inc.Title, &inc.Summary, &inc.Severity, &inc.Status, pq.Array(&inc.DeviceIDs), pq.Array(&inc.EventIDs), &inc.StartedAt, &inc.ResolvedAt, &inc.CreatedAt, &inc.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("incident not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get incident: %w", err)
	}

	return &inc, nil
}

func (r *IncidentRepository) List(ctx context.Context) ([]incident.Incident, error) {
	query := `
		SELECT id, title, summary, severity, status, device_ids, event_ids, started_at, resolved_at, created_at, updated_at
		FROM incidents
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	var incidents []incident.Incident
	for rows.Next() {
		var inc incident.Incident
		if err := rows.Scan(
			&inc.ID, &inc.Title, &inc.Summary, &inc.Severity, &inc.Status, pq.Array(&inc.DeviceIDs), pq.Array(&inc.EventIDs), &inc.StartedAt, &inc.ResolvedAt, &inc.CreatedAt, &inc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, inc)
	}

	return incidents, nil
}

func (r *IncidentRepository) Update(ctx context.Context, inc incident.Incident) (*incident.Incident, error) {
	query := `
		UPDATE incidents
		SET title = $2, summary = $3, severity = $4, status = $5, device_ids = $6, event_ids = $7, resolved_at = $8, updated_at = $9
		WHERE id = $1
		RETURNING id, title, summary, severity, status, device_ids, event_ids, started_at, resolved_at, created_at, updated_at
	`

	var updated incident.Incident
	err := r.db.QueryRowContext(
		ctx, query,
		inc.ID, inc.Title, inc.Summary, inc.Severity, inc.Status, pq.Array(inc.DeviceIDs), pq.Array(inc.EventIDs), inc.ResolvedAt, time.Now(),
	).Scan(
		&updated.ID, &updated.Title, &updated.Summary, &updated.Severity, &updated.Status, pq.Array(&updated.DeviceIDs), pq.Array(&updated.EventIDs), &updated.StartedAt, &updated.ResolvedAt, &updated.CreatedAt, &updated.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("update incident: %w", err)
	}

	return &updated, nil
}

func (r *IncidentRepository) Resolve(ctx context.Context, id string) error {
	query := `
		UPDATE incidents
		SET status = $2, resolved_at = $3, updated_at = $4
		WHERE id = $1
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, id, incident.StatusResolved, now, now)
	if err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}

	return nil
}
