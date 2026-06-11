package session

import (
	"context"
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	SessionID string          `json:"session_id"`
	DeviceID  string          `json:"device_id"`
	Type      string          `json:"event_type"`
	Severity  string          `json:"severity"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type Alert struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"`
	DeviceID   string     `json:"device_id"`
	Severity   string     `json:"severity"`
	Message    string     `json:"message"`
	Resolved   bool       `json:"resolved"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

type EventRepository interface {
	CreateEvent(ctx context.Context, e Event) (*Event, error)
	ListEventsBySession(ctx context.Context, sessionID string) ([]Event, error)
}

type AlertRepository interface {
	CreateAlert(ctx context.Context, a Alert) (*Alert, error)
	ResolveAlert(ctx context.Context, id string) error
	ListAlertsBySession(ctx context.Context, sessionID string) ([]Alert, error)
}

