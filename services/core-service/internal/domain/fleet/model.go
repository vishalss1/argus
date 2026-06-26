package fleet

import (
	"time"

	"github.com/vishalss1/argus/core/internal/domain/device"
)

// ponytail: minimal model without wifi_ssid/wifi_password storage as per requirements
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
