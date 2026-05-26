package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/event"
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

	return &created, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*event.Event, error) {
	query := `
		SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		FROM events
		WHERE id = $1
	`

	var ev event.Event
	err := r.db.QueryRowContext(ctx, query, id).Scan(
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

func (r *EventRepository) List(ctx context.Context) ([]event.Event, error) {
	query := `
		SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		FROM events
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
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

func (r *EventRepository) ListByDevice(ctx context.Context, deviceID string) ([]event.Event, error) {
	query := `
		SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		FROM events
		WHERE device_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, deviceID)
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

func (r *EventRepository) ListByType(ctx context.Context, eventType string) ([]event.Event, error) {
	query := `
		SELECT id, device_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		FROM events
		WHERE type = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, eventType)
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
