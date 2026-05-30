package ota

import "time"

const (
	StatusPending     = "pending"
	StatusAvailable   = "available"
	StatusDownloading = "downloading"
	StatusFlashing    = "flashing"
	StatusRebooting   = "rebooting"
	StatusAcked       = "acked"
	StatusNacked      = "nacked"
	StatusTimeout     = "timeout"
)

type FirmwareArtifact struct {
	ID             string    `json:"id" db:"id"`
	Version        string    `json:"version" db:"version"`
	Filename       string    `json:"filename" db:"filename"`
	ObjectKey      string    `json:"object_key" db:"object_key"`
	ContentType    string    `json:"content_type" db:"content_type"`
	SizeBytes      int64     `json:"size_bytes" db:"size_bytes"`
	ChecksumSHA256 string    `json:"checksum_sha256" db:"checksum_sha256"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

type Deployment struct {
	ID             string     `json:"id" db:"id"`
	DeviceID       string     `json:"device_id" db:"device_id"`
	ArtifactID     string     `json:"artifact_id" db:"artifact_id"`
	Status         string     `json:"status" db:"status"`
	Progress       int        `json:"progress" db:"progress"`
	ResultMessage  *string    `json:"result_message,omitempty" db:"result_message"`
	FailureReason  *string    `json:"failure_reason,omitempty" db:"failure_reason"`
	DeviceName     string     `json:"device_name,omitempty"`
	Version        string     `json:"version,omitempty"`
	Filename       string     `json:"filename,omitempty"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	AvailableAt    *time.Time `json:"available_at,omitempty" db:"available_at"`
	DownloadingAt  *time.Time `json:"downloading_at,omitempty" db:"downloading_at"`
	FlashingAt     *time.Time `json:"flashing_at,omitempty" db:"flashing_at"`
	RebootingAt    *time.Time `json:"rebooting_at,omitempty" db:"rebooting_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	FailedAt       *time.Time `json:"failed_at,omitempty" db:"failed_at"`
	TimedOutAt     *time.Time `json:"timed_out_at,omitempty" db:"timed_out_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}

type DeploymentEvent struct {
	ID           int64     `json:"id" db:"id"`
	DeploymentID string    `json:"deployment_id" db:"deployment_id"`
	DeviceID     string    `json:"device_id" db:"device_id"`
	Status       string    `json:"status" db:"status"`
	Progress     *int      `json:"progress,omitempty" db:"progress"`
	Message      *string   `json:"message,omitempty" db:"message"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type ProgressInput struct {
	DeploymentID string
	Status       string
	Progress     *int
	Message      string
}

type TimeoutPolicy struct {
	PendingTimeout time.Duration
	ActiveTimeout  time.Duration
}

type FleetStats struct {
	TotalDeployments      int     `json:"total_deployments"`
	SuccessfulDeployments int     `json:"successful_deployments"`
	FailedDeployments     int     `json:"failed_deployments"`
	SuccessRate           float64 `json:"success_rate"`
	DevicesPendingUpdate  int     `json:"devices_pending_update"`
}

type Manifest struct {
	DeploymentID   string    `json:"deployment_id"`
	DeviceID       string    `json:"device_id"`
	FirmwareID     string    `json:"firmware_id"`
	Version        string    `json:"version"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	SizeBytes      int64     `json:"size_bytes"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
	DownloadURL    string    `json:"download_url"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type UploadInput struct {
	Version     string
	Filename    string
	ContentType string
	SizeBytes   int64
}

type DeployInput struct {
	ArtifactID string
}

type ResultInput struct {
	Message string
}
