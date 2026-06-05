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

type StatisticsRepository interface {
	Upsert(ctx context.Context, s Statistics) error
	Get(ctx context.Context, sessionID string) (*Statistics, error)
}

type Artifact struct {
	SessionID     string          `json:"session_id"`
	WorkspaceID   string          `json:"workspace_id"`
	GeneratedAt   time.Time       `json:"generated_at"`
	ReportVersion string          `json:"report_version"`
	ArtifactJSON  json.RawMessage `json:"artifact_json"`
}

type SessionArtifactPayload struct {
	SessionID         string                               `json:"session_id"`
	GeneratedAt       string                               `json:"generated_at"`
	ReportVersion     string                               `json:"report_version"`
	WorkspaceID       string                               `json:"workspace_id"`
	SessionSummary    string                               `json:"session_summary"`
	DeviceSummaries   map[string]DeviceSummaryArtifact     `json:"device_summaries"`
	IncidentsArchive  []ArtifactIncident                   `json:"incidents_archive"`
	MetricsAggregates map[string]map[string]MetricAggregate `json:"metrics_aggregates"`
}

type DeviceSummaryArtifact struct {
	DeviceID              string `json:"device_id"`
	FirstSeen             string `json:"first_seen"`
	LastSeen              string `json:"last_seen"`
	SampleCount           int    `json:"sample_count"`
	WarningIncidentsCount int    `json:"warning_incidents_count"`
	CriticalIncidentsCount int   `json:"critical_incidents_count"`
	ActiveAtEnd           bool   `json:"active_at_end"`
}

type ArtifactIncident struct {
	DeviceID     string    `json:"device_id"`
	Metric       string    `json:"metric"`
	IncidentType string    `json:"incident_type"`
	Severity     string    `json:"severity"`
	StartTime    time.Time `json:"start_time"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	Occurrences  int       `json:"occurrences"`
	PeakScore    float64   `json:"peak_score"`
	Summary      string    `json:"summary"`
}

type MetricAggregate struct {
	Count    int     `json:"count"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Average  float64 `json:"average"`
	Variance float64 `json:"variance"`
}

func ParseArtifactPayload(data []byte) (*SessionArtifactPayload, error) {
	var payload SessionArtifactPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	// Apply defaults / compatibility behavior
	if payload.ReportVersion == "" {
		payload.ReportVersion = "1.0" // Default for older versions lacking a report version field
	}
	if payload.DeviceSummaries == nil {
		payload.DeviceSummaries = make(map[string]DeviceSummaryArtifact)
	}
	if payload.IncidentsArchive == nil {
		payload.IncidentsArchive = []ArtifactIncident{}
	}
	if payload.MetricsAggregates == nil {
		payload.MetricsAggregates = make(map[string]map[string]MetricAggregate)
	}

	return &payload, nil
}


