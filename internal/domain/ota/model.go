package ota

import "time"

const (
	StatusPending = "pending"
	StatusAcked   = "acked"
	StatusNacked  = "nacked"
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
	ResultMessage  *string    `json:"result_message,omitempty" db:"result_message"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty" db:"acknowledged_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
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
