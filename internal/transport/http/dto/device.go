package dto

import "encoding/json"

type CreateDeviceRequest struct {
	Name            string          `json:"name" validate:"required"`
	Type            string          `json:"type" validate:"required"`
	FirmwareVersion string          `json:"firmware_version"`
	Status          string          `json:"status,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type UpdateDeviceRequest struct {
	Name            *string          `json:"name,omitempty"`
	Type            *string          `json:"type,omitempty"`
	FirmwareVersion *string          `json:"firmware_version,omitempty"`
	Status          *string          `json:"status,omitempty"`
	Metadata        *json.RawMessage `json:"metadata,omitempty" swaggertype:"object"`
}

type HeartbeatRequest struct {
	Status string `json:"status,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
