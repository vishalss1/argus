package session

import (
	"context"
	"encoding/json"
	"time"
)

type Statistics struct {
	SessionID         string    `json:"session_id"`
	DurationSeconds   int       `json:"duration_seconds"`
	MessagesProcessed int       `json:"messages_processed"`
	AlertsCount       int       `json:"alerts_count"`
	CriticalEvents    int       `json:"critical_events"`
	UptimePercentage  float64   `json:"uptime_percentage"`
	AvgLatencyMS      float64   `json:"average_latency_ms"`
	UpdatedAt         time.Time `json:"updated_at"`
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
