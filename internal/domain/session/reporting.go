package session

import (
	"context"
	"encoding/json"
	"time"
)

type Statistics struct {
	SessionID                string    `json:"session_id"`
	DurationSeconds          int       `json:"duration_seconds"`
	MessagesProcessed        int       `json:"messages_processed"`
	AlertsCount              int       `json:"alerts_count"`
	CriticalEvents           int       `json:"critical_events"`
	UptimePercentage         float64   `json:"uptime_percentage"`
	AvgLatencyMS             float64   `json:"average_latency_ms"`
	AvgBattery               float64   `json:"average_battery"`
	MinBattery               float64   `json:"minimum_battery"`
	MaxBattery               float64   `json:"maximum_battery"`
	AvgTemperature           float64   `json:"average_temperature"`
	MinTemperature           float64   `json:"minimum_temperature"`
	MaxTemperature           float64   `json:"maximum_temperature"`
	DistanceTravelled        float64   `json:"distance_travelled"`
	DeviceParticipationCount int       `json:"device_participation_count"`
	CommandCount             int       `json:"command_count"`
	AnomalyCount             int       `json:"anomaly_count"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type Report struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	ReportJSON  json.RawMessage `json:"report_json"`
	GeneratedAt time.Time       `json:"generated_at"`
}

type StatisticsRepository interface {
	Upsert(ctx context.Context, s Statistics) error
	Get(ctx context.Context, sessionID string) (*Statistics, error)
}

type ReportRepository interface {
	Create(ctx context.Context, r Report) (*Report, error)
	GetBySession(ctx context.Context, sessionID string) (*Report, error)
}
