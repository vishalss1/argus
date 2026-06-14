package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	common "github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/telemetry/internal/domain/event"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/ai"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) Create(ctx context.Context, ev event.Event) (*event.Event, error) {
	wID, _ := common.GetWorkspaceID(ctx)
	if wID == "" && ev.WorkspaceID != "" {
		wID = ev.WorkspaceID
	}

	query := `
		INSERT INTO events (
			id, device_id, workspace_id, type, severity, title, summary, source, confidence_score, metadata, created_at
		) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
	`

	var created event.Event
	var wIDStr string
	err := r.db.QueryRowContext(
		ctx, query,
		ev.ID, ev.DeviceID, wID, ev.Type, ev.Severity, ev.Title, ev.Summary, ev.Source, ev.ConfidenceScore, ev.Metadata, ev.CreatedAt,
	).Scan(
		&created.ID, &created.DeviceID, &wIDStr, &created.Type, &created.Severity, &created.Title, &created.Summary, &created.Source, &created.ConfidenceScore, &created.Metadata, &created.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", err)
	}
	created.WorkspaceID = wIDStr

	ai.EventsGeneratedTotal.WithLabelValues(created.Type, string(created.Severity)).Inc()

	return &created, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*event.Event, error) {
	var query string
	var row *sql.Row

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE id = $1 AND workspace_id = $2::uuid
		`
		row = r.db.QueryRowContext(ctx, query, id, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE id = $1
		`
		row = r.db.QueryRowContext(ctx, query, id)
	}

	var ev event.Event
	var wIDStr string
	err := row.Scan(
		&ev.ID, &ev.DeviceID, &wIDStr, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	ev.WorkspaceID = wIDStr

	return &ev, nil
}

func (r *EventRepository) List(ctx context.Context, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE workspace_id = $3::uuid
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		rows, err = r.db.QueryContext(ctx, query, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
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
		var wIDStr string
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &wIDStr, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.WorkspaceID = wIDStr
		events = append(events, ev)
	}

	return events, nil
}

func (r *EventRepository) ListByDevice(ctx context.Context, deviceID string, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE device_id = $1 AND workspace_id = $4::uuid
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, deviceID, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
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
		var wIDStr string
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &wIDStr, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.WorkspaceID = wIDStr
		events = append(events, ev)
	}

	return events, nil
}

func (r *EventRepository) ListByType(ctx context.Context, eventType string, limit, offset int) ([]event.Event, error) {
	var query string
	var rows *sql.Rows
	var err error

	if wID, ok := common.GetWorkspaceID(ctx); ok {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
			FROM events
			WHERE type = $1 AND workspace_id = $4::uuid
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
		rows, err = r.db.QueryContext(ctx, query, eventType, limit, offset, wID)
	} else {
		query = `
			SELECT id, device_id, COALESCE(workspace_id::text, ''), type, severity, title, summary, source, confidence_score, metadata, created_at
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
		var wIDStr string
		if err := rows.Scan(
			&ev.ID, &ev.DeviceID, &wIDStr, &ev.Type, &ev.Severity, &ev.Title, &ev.Summary, &ev.Source, &ev.ConfidenceScore, &ev.Metadata, &ev.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		ev.WorkspaceID = wIDStr
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
