package dto

import "encoding/json"

type CreateDeviceRequest struct {
	ID              string          `json:"id,omitempty"`
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
	DeviceID        string `json:"device_id,omitempty"`
	Status          string `json:"status,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
}

type ProvisionDeviceRequest struct {
	HardwareID      string          `json:"hardware_id" validate:"required"`
	DeviceType      string          `json:"device_type" validate:"required"`
	FirmwareVersion string          `json:"firmware_version"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty" swaggertype:"object"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

