package model

import (
	"encoding/json"
	"time"
)

type Device struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	FirmwareVersion string          `json:"firmware_version"`
	Status          string          `json:"status"`
	Metadata        json.RawMessage `json:"metadata"`
	LastSeen        *time.Time      `json:"last_seen,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type CreateDeviceRequest struct {
	Name            string          `json:"name"`
	Type            string          `json:"type"`
	FirmwareVersion string          `json:"firmware_version"`
	Status          string          `json:"status,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type UpdateDeviceRequest struct {
	Name            *string          `json:"name,omitempty"`
	Type            *string          `json:"type,omitempty"`
	FirmwareVersion *string          `json:"firmware_version,omitempty"`
	Status          *string          `json:"status,omitempty"`
	Metadata        *json.RawMessage `json:"metadata,omitempty"`
}
