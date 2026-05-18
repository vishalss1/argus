package command

import (
	"encoding/json"
	"time"
)

const (
	StatusPending = "pending"
	StatusAcked   = "acked"
	StatusNacked  = "nacked"
)

type Command struct {
	ID             string          `json:"id" db:"id"`
	DeviceID       string          `json:"device_id" db:"device_id"`
	Type           string          `json:"type" db:"command_type"`
	Payload        json.RawMessage `json:"payload" db:"payload" swaggertype:"object"`
	Status         string          `json:"status" db:"status"`
	ResultMessage  *string         `json:"result_message,omitempty" db:"result_message"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	SentAt         *time.Time      `json:"sent_at,omitempty" db:"sent_at"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}

type SendInput struct {
	Type    string
	Payload json.RawMessage
}

type ResultInput struct {
	Message string
}
