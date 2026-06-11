package device

import "time"

type Device struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	FirmwareVersion string     `json:"firmware_version"`
	Status          string     `json:"status"`
	WorkspaceID     *string    `json:"workspace_id,omitempty"`
	LastSeen        *time.Time `json:"last_seen,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
