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

type Artifact struct {
	SessionID     string          `json:"session_id"`
	WorkspaceID   string          `json:"workspace_id"`
	GeneratedAt   time.Time       `json:"generated_at"`
	ReportVersion string          `json:"report_version"`
	ArtifactJSON  json.RawMessage `json:"artifact_json"`
}

type SessionArtifactPayload struct {
	SessionID        string                         `json:"session_id"`
	GeneratedAt      string                         `json:"generated_at"`
	ReportVersion    string                         `json:"report_version"`
	WorkspaceID      string                         `json:"workspace_id"`
	SessionSummary   string                         `json:"session_summary"`
	DeviceSummaries  map[string]DeviceSummaryReport `json:"device_summaries"`
	Alerts           []AlertArchive                 `json:"alerts"`
	Commands         []CommandArchive               `json:"commands"`
	AIFindings       []AIFindingsArchive            `json:"ai_findings"`
	Timeline         []TimelineEntry                `json:"timeline"`
	TelemetryRollups map[string][]TelemetryRollup  `json:"telemetry_rollups"`
}

type DeviceSummaryReport struct {
	DeviceID         string  `json:"device_id"`
	FirstSeen        string  `json:"first_seen"`
	LastSeen         string  `json:"last_seen"`
	UptimePercentage float64 `json:"uptime_percentage"`
	SampleCount      int     `json:"sample_count"`

	BatteryAverage float64 `json:"battery_average"`
	BatteryMin     float64 `json:"battery_min"`
	BatteryMax     float64 `json:"battery_max"`

	TemperatureAverage float64 `json:"temperature_average"`
	TemperatureMin     float64 `json:"temperature_min"`
	TemperatureMax     float64 `json:"temperature_max"`

	SignalAverage float64 `json:"signal_average"`
	SignalMin     float64 `json:"signal_min"`
	SignalMax     float64 `json:"signal_max"`

	DistanceTravelled float64 `json:"distance_travelled"`

	WarningCount  int `json:"warning_count"`
	CriticalCount int `json:"critical_count"`

	CommandsReceived  int `json:"commands_received"`
	AnomaliesDetected int `json:"anomalies_detected"`
}

type AlertArchive struct {
	Timestamp       string `json:"timestamp"`
	Severity        string `json:"severity"`
	SourceDevice    string `json:"source_device"`
	AlertType       string `json:"alert_type"`
	Message         string `json:"message"`
	ResolutionState string `json:"resolution_state"`
}

type CommandArchive struct {
	Timestamp           string  `json:"timestamp"`
	TargetDevice        string  `json:"target_device"`
	Command             string  `json:"command"`
	Status              string  `json:"status"`
	AcknowledgementTime *string `json:"acknowledgement_time,omitempty"`
}

type AIFindingsArchive struct {
	Timestamp       string  `json:"timestamp"`
	DeviceID        string  `json:"device_id"`
	FindingType     string  `json:"finding_type"`
	Severity        string  `json:"severity"`
	Recommendation  string  `json:"recommendation"`
	ConfidenceScore float64 `json:"confidence_score"`
}

type TimelineEntry struct {
	Timestamp string  `json:"timestamp"`
	Type      string  `json:"type"`
	DeviceID  *string `json:"device_id,omitempty"`
	Message   string  `json:"message"`
}

type TelemetryRollup struct {
	Timestamp      string  `json:"timestamp"`
	BatteryAvg     float64 `json:"battery_avg"`
	BatteryMin     float64 `json:"battery_min"`
	BatteryMax     float64 `json:"battery_max"`
	TemperatureAvg float64 `json:"temperature_avg"`
	TemperatureMin float64 `json:"temperature_min"`
	TemperatureMax float64 `json:"temperature_max"`
	SignalAvg      float64 `json:"signal_avg"`
	SignalMin      float64 `json:"signal_min"`
	SignalMax      float64 `json:"signal_max"`
	SampleCount    int     `json:"sample_count"`
}

