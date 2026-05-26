package anomaly

import (
	"encoding/json"
	"time"
)

type AnomalyType string

const (
	AnomalyTypeConnectivity AnomalyType = "connectivity"
	AnomalyTypeLatency      AnomalyType = "latency"
	AnomalyTypeThermal      AnomalyType = "thermal"
	AnomalyTypeBattery      AnomalyType = "battery"
	AnomalyTypeTelemetry    AnomalyType = "telemetry"
)

type Anomaly struct {
	ID              string          `json:"id"`
	DeviceID        string          `json:"device_id"`
	Type            AnomalyType     `json:"type"`
	Severity        string          `json:"severity"`
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	ConfidenceScore float64         `json:"confidence_score"`
	Metadata        json.RawMessage `json:"metadata"`
	DetectedAt      time.Time       `json:"detected_at"`
	CreatedAt       time.Time       `json:"created_at"`
}
