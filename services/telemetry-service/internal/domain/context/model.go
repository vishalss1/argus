package context

import (
	"encoding/json"
	"time"
)

type MemoryType string

const (
	MemoryTypeIncident          MemoryType = "incident"
	MemoryTypeAnomaly           MemoryType = "anomaly"
	MemoryTypeDeployment        MemoryType = "deployment"
	MemoryTypeConnectivity      MemoryType = "connectivity"
	MemoryTypeCommandOutcome    MemoryType = "command_outcome"
	MemoryTypeBehavioralSummary MemoryType = "behavioral_summary"
)

type OperationalMemory struct {
	ID          string          `json:"id"`
	DeviceID    *string         `json:"device_id,omitempty"`
	WorkspaceID string          `json:"workspace_id,omitempty"`
	Type        MemoryType      `json:"type"`
	Summary   string          `json:"summary"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
	CreatedAt time.Time       `json:"created_at"`
}
