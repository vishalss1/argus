package policy

import (
	"time"
)

type ActionType string

const (
	ActionReboot           ActionType = "reboot"
	ActionRestartService   ActionType = "restart_service"
	ActionRollbackFirmware ActionType = "rollback_firmware"
	ActionIsolateNode      ActionType = "isolate_node"
	ActionUpdateConfig     ActionType = "update_config"
)

type Policy struct {
	ID              string       `json:"id"`
	Action          ActionType   `json:"action"`
	AllowedDevices  []string     `json:"allowed_devices"`  // Empty means all
	RequiresApproval bool         `json:"requires_approval"`
	MaxPerDay       int          `json:"max_per_day"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type ExecutionRecord struct {
	ID          string     `json:"id"`
	Action      ActionType `json:"action"`
	DeviceID    string     `json:"device_id"`
	IncidentID  *string    `json:"incident_id,omitempty"`
	Status      string     `json:"status"` // pending, approved, rejected, executed, failed
	SuggestedBy string     `json:"suggested_by"`
	ApprovedBy  *string    `json:"approved_by,omitempty"`
	Metadata    string     `json:"metadata"`
	CreatedAt   time.Time  `json:"created_at"`
}
