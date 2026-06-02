package finding

import (
	"context"
	"encoding/json"
	"time"
)

type Finding struct {
	ID          string          `json:"id"`
	DeviceID    string          `json:"device_id"`
	RiskScore   float64         `json:"risk_score"`
	HealthScore int             `json:"health_score"`
	Severity    string          `json:"severity"`
	Summary     string          `json:"summary"`
	Metadata    json.RawMessage `json:"metadata"`
	CreatedAt   time.Time       `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, f Finding) (*Finding, error)
	ListByDevice(ctx context.Context, deviceID string) ([]Finding, error)
}
