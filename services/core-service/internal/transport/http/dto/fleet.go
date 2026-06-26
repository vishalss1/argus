package dto

import (
	"time"

	"github.com/vishalss1/argus/core/internal/domain/device"
)

type CreateFleetRequest struct {
	Name             string `json:"name" validate:"required"`
	NodeRole         string `json:"node_role"`
	HardwareType     string `json:"hardware_type" validate:"required"`
	NodePrefix       string `json:"node_prefix"`
	NodeCount        int    `json:"node_count" validate:"min=1,max=500"`
	FirmwareVersion  string `json:"firmware_version"`
	FirmwareTemplate string `json:"firmware_template"`
	WiFiSSID         string `json:"wifi_ssid"`
	WiFiPassword     string `json:"wifi_password"`
}

type FleetResponse struct {
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	Name             string          `json:"name"`
	NodeRole         string          `json:"node_role"`
	HardwareType     string          `json:"hardware_type"`
	NodePrefix       string          `json:"node_prefix"`
	FirmwareVersion  string          `json:"firmware_version"`
	FirmwareTemplate string          `json:"firmware_template"`
	NodeCount        int             `json:"node_count"`
	TotalNodes       int             `json:"total_nodes"`
	OnlineNodes      int             `json:"online_nodes"`
	OfflineNodes     int             `json:"offline_nodes"`
	Devices          []device.Device `json:"devices,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
