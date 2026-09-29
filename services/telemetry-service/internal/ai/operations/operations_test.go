package operations

import (
	"testing"
	"time"

	"github.com/vishalss1/argus/telemetry/internal/domain/device"
)

func TestClassifyIntent(t *testing.T) {
	tests := map[string]Intent{
		"Summarize device X":                   IntentDeviceSummary,
		"How is device X doing?":               IntentDeviceSummary,
		"Why did device X fail?":               IntentRootCauseAnalysis,
		"What caused the warning?":             IntentRootCauseAnalysis,
		"How do I fix device X?":               IntentRemediation,
		"Tell me about free_heap incident":     IntentIncidentLookup,
		"What happened in the fleet today?":    IntentFleetSummary,
		"Which devices show similar patterns?": IntentDeviceComparison,
	}
	for query, expected := range tests {
		if actual := ClassifyIntent(query); actual != expected {
			t.Fatalf("ClassifyIntent(%q) = %s, want %s", query, actual, expected)
		}
	}
}

func TestDeviceSummaryAndRootCauseForStalledTelemetry(t *testing.T) {
	now := time.Now().UTC()
	snapshot := Snapshot{
		Device:              device.Device{ID: "device-1", Name: "Device X", Status: "online"},
		TelemetryRecordedAt: &now,
		IncidentHistory: []Incident{{
			DeviceID: "device-1", Metric: "free_heap", IncidentType: "numeric_stuck",
			Severity: "warning", Status: "open", StartTime: now.Add(-16 * time.Minute),
			LastSeen: now, Occurrences: 10, PeakScore: 1,
		}},
	}

	summary := NewDeviceSummaryAnalyzer().Analyze(snapshot)
	if summary.OpenIncidents != 1 || summary.HealthScore >= 100 || summary.Severity != "warning" {
		t.Fatalf("unexpected summary: %#v", summary)
	}

	rca := NewRootCauseAnalyzer().Analyze(snapshot)
	if rca.Confidence < 80 {
		t.Fatalf("expected high-confidence root cause, got %#v", rca)
	}
	if rca.PrimaryCause == "" || len(rca.RecommendedActions) == 0 {
		t.Fatalf("root cause analysis is incomplete: %#v", rca)
	}
}
