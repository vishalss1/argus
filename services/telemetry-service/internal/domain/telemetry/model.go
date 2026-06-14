package telemetry

import (
	"encoding/json"
	"time"
)

type Telemetry struct {
	ID         string          `json:"id" db:"id"`
	DeviceID   string          `json:"device_id" db:"device_id"`
	RecordedAt time.Time       `json:"recorded_at" db:"recorded_at"`
	Metrics    json.RawMessage `json:"metrics" db:"metrics" swaggertype:"object"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

type CreateInput struct {
	RecordedAt *time.Time
	Metrics    json.RawMessage
}
