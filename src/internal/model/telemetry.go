package model

import (
	"encoding/json"
	"time"
)

type Telemetry struct {
	ID         string          `json:"id"`
	DeviceID   string          `json:"device_id"`
	RecordedAt time.Time       `json:"recorded_at"`
	Metrics    json.RawMessage `json:"metrics"`
	CreatedAt  time.Time       `json:"created_at"`
}

type CreateTelemetryRequest struct {
	RecordedAt *time.Time      `json:"recorded_at,omitempty"`
	Metrics    json.RawMessage `json:"metrics"`
}
