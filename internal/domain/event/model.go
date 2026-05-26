package event

import (
	"encoding/json"
	"time"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

type Event struct {
	ID              string          `json:"id"`
	DeviceID        string          `json:"device_id"`
	Type            string          `json:"type"`
	Severity        Severity        `json:"severity"`
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	Source          string          `json:"source"`
	ConfidenceScore float64         `json:"confidence_score"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
}
