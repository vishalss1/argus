package fleet

import (
	"time"

	"github.com/vishalss1/argus/core/internal/domain/device"
)

// Fleet representation without wifi_ssid/wifi_password storage as per requirements
type Fleet struct {
	ID               string
	WorkspaceID      string
	Name             string
	NodeRole         string
	HardwareType     string
	NodePrefix       string
	FirmwareVersion  string
	FirmwareTemplate string
	NodeCount        int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateFleetInput struct {
	Name             string
	NodeRole         string
	HardwareType     string
	NodePrefix       string
	NodeCount        int
	FirmwareVersion  string
	FirmwareTemplate string
	WiFiSSID         string
	WiFiPassword     string
}

type FleetWithStats struct {
	Fleet
	TotalNodes   int
	OnlineNodes  int
	OfflineNodes int
	Devices      []device.Device
}

type FleetProvisionResult struct {
	Fleet   Fleet
	ZipData []byte
}

type FleetDeployResult struct {
	FleetID       string             `json:"fleet_id"`
	ArtifactID    string             `json:"artifact_id"`
	DeployedCount int                `json:"deployed_count"`
	TotalCount    int                `json:"total_count"`
	Errors        []FleetDeployError `json:"errors,omitempty"`
}

type FleetDeployError struct {
	DeviceID string `json:"device_id"`
	Error    string `json:"error"`
}
