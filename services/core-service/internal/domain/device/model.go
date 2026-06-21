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
	WorkspaceID     *string         `json:"workspace_id,omitempty" db:"workspace_id"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`

	APIKeyHash   []byte  `json:"-" db:"api_key_hash"`
	APIKeyPrefix *string `json:"-" db:"api_key_prefix"`
	RawAPIKey    *string `json:"api_key,omitempty" db:"-"`
}

type CreateInput struct {
	ID              string
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
	APIKeyHash      []byte
	APIKeyPrefix    *string
}

type HeartbeatInput struct {
	Status          string
	FirmwareVersion string
}

type PresenceStatus string

const (
	PresenceOnline  PresenceStatus = "online"
	PresenceOffline PresenceStatus = "offline"
)

type PresenceState struct {
	DeviceID      string         `json:"deviceId"`
	Online        bool           `json:"online"`
	Status        PresenceStatus `json:"status"`
	LastSeen      time.Time      `json:"lastSeen"`
	ConnectedAt   *time.Time     `json:"connectedAt,omitempty"`
	LastHeartbeat *time.Time     `json:"lastHeartbeat,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type PresenceInput struct {
	Status    PresenceStatus
	Timestamp time.Time
	Metadata  map[string]any
}

type PresenceEvent struct {
	Type      string `json:"type"`
	DeviceID  string `json:"deviceId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
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

