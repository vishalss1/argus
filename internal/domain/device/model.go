package device

import (
	"encoding/json"
	"time"
)

type Device struct {
	ID              string          `json:"id" db:"id"`
	Name            string          `json:"name" db:"name"`
	Type            string          `json:"type" db:"type"`
	FirmwareVersion string          `json:"firmware_version" db:"firmware_version"`
	Status          string          `json:"status" db:"status"`
	Metadata        json.RawMessage `json:"metadata" db:"metadata" swaggertype:"object"`
	LastSeen        *time.Time      `json:"last_seen,omitempty" db:"last_seen"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

type CreateInput struct {
	Name            string
	Type            string
	FirmwareVersion string
	Status          string
	Metadata        json.RawMessage
}

type UpdateInput struct {
	Name            *string
	Type            *string
	FirmwareVersion *string
	Status          *string
	Metadata        *json.RawMessage
}

type HeartbeatInput struct {
	Status string
}

type ProvisionInput struct {
	HardwareID      string
	DeviceType      string
	FirmwareVersion string
	Capabilities    json.RawMessage
}

type ProvisioningConfig struct {
	MQTTBrokerURL        string
	MQTTTelemetryPattern string
	SamplingIntervalMS   int
	HeartbeatIntervalMS  int
}

type ProvisionResponse struct {
	DeviceUUID          string `json:"device_uuid"`
	MQTTBrokerURL       string `json:"mqtt_broker_url"`
	MQTTTelemetryTopic  string `json:"mqtt_telemetry_topic"`
	MQTTCommandTopic    string `json:"mqtt_command_topic"`
	SamplingIntervalMS  int    `json:"sampling_interval_ms"`
	HeartbeatIntervalMS int    `json:"heartbeat_interval_ms"`
}
