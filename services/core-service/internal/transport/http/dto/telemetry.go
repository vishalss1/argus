package dto

import (
	"encoding/json"
	"time"
)

type CreateTelemetryRequest struct {
	RecordedAt *time.Time      `json:"recorded_at,omitempty"`
	Metrics    json.RawMessage `json:"metrics" validate:"required" swaggertype:"object"`
}

