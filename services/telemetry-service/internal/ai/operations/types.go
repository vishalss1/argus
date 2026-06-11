package operations

import (
	"encoding/json"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/domain/device"
)

type Intent string

const (
	IntentDeviceSummary     Intent = "DEVICE_SUMMARY"
	IntentRootCauseAnalysis Intent = "ROOT_CAUSE_ANALYSIS"
	IntentRemediation       Intent = "REMEDIATION"
	IntentIncidentLookup    Intent = "INCIDENT_LOOKUP"
	IntentFleetSummary      Intent = "FLEET_SUMMARY"
	IntentDeviceComparison  Intent = "DEVICE_COMPARISON"
	IntentUnknown           Intent = "UNKNOWN"
)

type Incident struct {
	DeviceID     string    `json:"device_id"`
	Metric       string    `json:"metric"`
	IncidentType string    `json:"incident_type"`
	Severity     string    `json:"severity"`
	Status       string    `json:"status"`
	StartTime    time.Time `json:"start_time"`
	LastSeen     time.Time `json:"last_seen"`
	Occurrences  int       `json:"occurrences"`
	PeakScore    float64   `json:"peak_score"`
	Summary      string    `json:"summary"`
}

type MetricTrend struct {
	Metric    string  `json:"metric"`
	Current   any     `json:"current,omitempty"`
	Count     int64   `json:"count"`
	Minimum   float64 `json:"minimum,omitempty"`
	Maximum   float64 `json:"maximum,omitempty"`
	Average   float64 `json:"average,omitempty"`
	Variance  float64 `json:"variance,omitempty"`
	Direction string  `json:"direction"`
	Anomalous bool    `json:"anomalous"`
}

type HistoryItem struct {
	Type      string          `json:"type"`
	Summary   string          `json:"summary"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type TelemetryPoint struct {
	RecordedAt time.Time      `json:"recordedAt"`
	Metrics    map[string]any `json:"metrics"`
}

type Snapshot struct {
	Device              device.Device               `json:"device"`
	LatestTelemetry     map[string]any              `json:"latestTelemetry"`
	TelemetryRecordedAt *time.Time                  `json:"telemetryRecordedAt,omitempty"`
	TelemetryWindows    map[string][]TelemetryPoint `json:"telemetryWindows"`
	TelemetryTrends     []MetricTrend               `json:"telemetryTrends"`
	AnomalyHistory      []HistoryItem               `json:"anomalyHistory"`
	IncidentHistory     []Incident                  `json:"incidentHistory"`
	ConnectivityHistory []HistoryItem               `json:"connectivityHistory"`
	FirmwareInfo        map[string]any              `json:"firmwareInfo"`
	GeneratedAnalysis   map[string]any              `json:"generatedAnalysis,omitempty"`
}

type DeviceSummary struct {
	DeviceID         string   `json:"deviceId"`
	DeviceName       string   `json:"deviceName"`
	DeviceStatus     string   `json:"deviceStatus"`
	Severity         string   `json:"severity"`
	OpenIncidents    int      `json:"openIncidents"`
	RecentIncidents  int      `json:"recentIncidents"`
	IncidentsLast24h int      `json:"incidentsLast24h"`
	IncidentsLast7d  int      `json:"incidentsLast7d"`
	HealthScore      int      `json:"healthScore"`
	KeyFindings      []string `json:"keyFindings"`
	Recommendations  []string `json:"recommendations"`
}

type RootCauseAnalysis struct {
	Confidence         int      `json:"confidence"`
	PrimaryCause       string   `json:"primaryCause"`
	SupportingEvidence []string `json:"supportingEvidence"`
	AlternativeCauses  []string `json:"alternativeCauses"`
	RecommendedActions []string `json:"recommendedActions"`
}

type Remediation struct {
	Pattern        string   `json:"pattern"`
	Reasoning      string   `json:"reasoning"`
	PossibleCauses []string `json:"possibleCauses"`
	Actions        []string `json:"actions"`
	Metric         string   `json:"metric,omitempty"`
	DeviceID       string   `json:"deviceId,omitempty"`
	Severity       string   `json:"severity,omitempty"`
}

type RelatedDevice struct {
	DeviceID       string   `json:"deviceId"`
	DeviceName     string   `json:"deviceName"`
	Similarity     int      `json:"similarity"`
	SharedPatterns []string `json:"sharedPatterns"`
}
