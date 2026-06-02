package telemetry

import (
	"context"
	"time"
)

type ExportFormat string

const (
	ExportFormatCSV  ExportFormat = "csv"
	ExportFormatJSON ExportFormat = "json"
)

type ExportRequest struct {
	DeviceID string       `json:"device_id"`
	Format   ExportFormat `json:"format"`
	From     time.Time    `json:"from"`
	To       time.Time    `json:"to"`
}

type ExportResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type ExportService interface {
	Export(ctx context.Context, req ExportRequest) (*ExportResponse, error)
}
