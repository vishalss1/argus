package telemetry

import (
	"context"
	"encoding/json"
	"time"
)

type FleetSummary struct {
	ID               string          `json:"id"`
	ActiveDevices    int             `json:"active_devices"`
	OfflineDevices   int             `json:"offline_devices"`
	AvgHealthScore   float64         `json:"avg_health_score"`
	AvgRiskScore     float64         `json:"avg_risk_score"`
	HighRiskDevices  int             `json:"high_risk_devices"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
}

type FleetRepository interface {
	Create(ctx context.Context, s FleetSummary) (*FleetSummary, error)
	GetLatest(ctx context.Context) (*FleetSummary, error)
	List(ctx context.Context, limit int) ([]FleetSummary, error)
}
