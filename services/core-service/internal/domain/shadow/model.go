package shadow

import (
	"encoding/json"
	"time"
)

type Shadow struct {
	DeviceID  string          `json:"device_id"`
	Desired   json.RawMessage `json:"desired" swaggertype:"object"`
	Reported  json.RawMessage `json:"reported" swaggertype:"object"`
	Drift     bool            `json:"drift"`
	Version   int64           `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type UpdateInput struct {
	State json.RawMessage
}

