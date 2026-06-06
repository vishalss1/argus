package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/auth"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(ctx context.Context, ev event.Event) (*event.Event, error) {
	query := `
		INSERT INTO events (
			id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
	`

	var created event.Event
	err := r.db.QueryRowContext(
		ctx, query,
		ev.ID, ev.DeviceID, ev.Type, ev.Severity, ev.Title, ev.Summary, ev.Source, ev.ConfidenceScore, ev.Metadata, ev.CreatedAt,
	).Scan(
		&created.ID, &created.DeviceID, &created.Type, &created.Severity, &created.Title, &created.Summary, &created.Source, &created.ConfidenceScore, &created.Metadata, &created.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}

	ai.EventsGeneratedTotal.WithLabelValues(created.Type, string(created.Severity)).Inc()

	return &created, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*event.Event, error) {
	var query string
	var row *sql.Row

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT e.id, e.device_id, e.type, e.severity, e.title, e.summary, e.source, e.confidence_score, e.metadata, e.created_at
			FROM events e
			JOIN devices d ON e.device_id = d.id
			WHERE e.id = $1 AND d.workspace_id = $2::uuid
		`
		row = r.db.QueryRowContext(ctx, query, id, wID)
	} else {
		query = `
			SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE id = $1
		`
		row = r.db.QueryRowContext(ctx, query, id)
	}

	var ev event.Event
	err := row.Scan(
		&ev.ID, &ev.DeviceID, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}

	return &ev, nil
}

func (r *EventRepository) List(ctx context.Context, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT e.id, e.device_id, e.type, e.severity, e.title, e.summary, e.source, e.confidence_score, e.metadata, e.created_at
			FROM events e
			JOIN devices d ON e.device_id = d.id
			WHERE d.workspace_id = $3::uuid
			ORDER BY e.created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, err = r.db.QueryContext(ctx, query, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, err = r.db.QueryContext(ctx, query, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []event.Event
	for rows.Next() {
		var ev event.Event
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, ev)
	}

	return events, nil
}

func (r *EventRepository) ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT e.id, e.device_id, e.type, e.severity, e.title, e.summary, e.source, e.confidence_score, e.metadata, e.created_at
			FROM events e
			JOIN devices d ON e.device_id = d.id
			WHERE e.device_id = $1 AND d.workspace_id = $4::uuid
			ORDER BY e.created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE device_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("list events by device: %w", err)
	}
	defer rows.Close()

	var events []event.Event
	for rows.Next() {
		var ev event.Event
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, ev)
	}

	return events, nil
}

func (r *EventRepository) ListByType(ctx context.Context, eventType string, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := auth.GetWorkspaceID(ctx); ok {
		query = `
			SELECT e.id, e.device_id, e.type, e.severity, e.title, e.summary, e.source, e.confidence_score, e.metadata, e.created_at
			FROM events e
			JOIN devices d ON e.device_id = d.id
			WHERE e.type = $1 AND d.workspace_id = $4::uuid
			ORDER BY e.created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, eventType, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE type = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, eventType, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("list events by type: %w", err)
	}
	defer rows.Close()

	var events []event.Event
	for rows.Next() {
		var ev event.Event
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, ev)
	}

	return events, nil
}

func (r *EventRepository) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM events WHERE created_at < $1`
	res, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}
