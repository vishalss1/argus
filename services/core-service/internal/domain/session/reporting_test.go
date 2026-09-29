package session_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/session"
)

// 1. & 2. Serialization & Deserialization (Round-trip)
func TestSessionArtifactRoundTrip(t *testing.T) {
	resolvedAt := time.Now().UTC().Add(30 * time.Minute)
	original := session.SessionArtifactPayload{
		SessionID:      "test-session-id",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ReportVersion:  "3.0",
		WorkspaceID:    "test-workspace-id",
		SessionSummary: "A clean run with no incidents.",
		DeviceSummaries: map[string]session.DeviceSummaryArtifact{
			"dev-1": {
				DeviceID:               "dev-1",
				FirstSeen:              time.Now().UTC().Format(time.RFC3339),
				LastSeen:               time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
				SampleCount:            100,
				WarningIncidentsCount:  0,
				CriticalIncidentsCount: 0,
				ActiveAtEnd:            false,
			},
		},
		IncidentsArchive: []session.ArtifactIncident{
			{
				DeviceID:     "dev-1",
				Metric:       "temperature",
				IncidentType: "high_temp",
				Severity:     "warning",
				StartTime:    time.Now().UTC(),
				ResolvedAt:   &resolvedAt,
				Occurrences:  1,
				PeakScore:    0.85,
				Summary:      "High temp warning",
			},
		},
		MetricsAggregates: map[string]map[string]session.MetricAggregate{
			"dev-1": {
				"temperature": {
					Count:    100,
					Min:      20.5,
					Max:      35.2,
					Average:  24.8,
					Variance: 1.25,
				},
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal artifact: %v", err)
	}

	parsed, err := session.ParseArtifactPayload(data)
	if err != nil {
		t.Fatalf("failed to parse artifact: %v", err)
	}

	if parsed.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: got %v, want %v", parsed.SessionID, original.SessionID)
	}
	if parsed.GeneratedAt != original.GeneratedAt {
		t.Errorf("GeneratedAt mismatch: got %v, want %v", parsed.GeneratedAt, original.GeneratedAt)
	}
	if parsed.ReportVersion != original.ReportVersion {
		t.Errorf("ReportVersion mismatch: got %v, want %v", parsed.ReportVersion, original.ReportVersion)
	}
	if parsed.WorkspaceID != original.WorkspaceID {
		t.Errorf("WorkspaceID mismatch: got %v, want %v", parsed.WorkspaceID, original.WorkspaceID)
	}
	if parsed.SessionSummary != original.SessionSummary {
		t.Errorf("SessionSummary mismatch: got %v, want %v", parsed.SessionSummary, original.SessionSummary)
	}

	// Verify device summaries
	if len(parsed.DeviceSummaries) != len(original.DeviceSummaries) {
		t.Fatalf("DeviceSummaries length mismatch: got %d, want %d", len(parsed.DeviceSummaries), len(original.DeviceSummaries))
	}
	dOrig := original.DeviceSummaries["dev-1"]
	dParsed := parsed.DeviceSummaries["dev-1"]
	if dParsed.DeviceID != dOrig.DeviceID || dParsed.SampleCount != dOrig.SampleCount || dParsed.ActiveAtEnd != dOrig.ActiveAtEnd {
		t.Errorf("DeviceSummary mismatch: got %+v, want %+v", dParsed, dOrig)
	}

	// Verify incidents
	if len(parsed.IncidentsArchive) != len(original.IncidentsArchive) {
		t.Fatalf("IncidentsArchive length mismatch: got %d, want %d", len(parsed.IncidentsArchive), len(original.IncidentsArchive))
	}
	iOrig := original.IncidentsArchive[0]
	iParsed := parsed.IncidentsArchive[0]
	if iParsed.DeviceID != iOrig.DeviceID || iParsed.Metric != iOrig.Metric || iParsed.IncidentType != iOrig.IncidentType || iParsed.Summary != iOrig.Summary {
		t.Errorf("IncidentArchive mismatch: got %+v, want %+v", iParsed, iOrig)
	}
	if iParsed.ResolvedAt == nil || *iParsed.ResolvedAt != *iOrig.ResolvedAt {
		t.Errorf("Incident resolved_at mismatch: got %v, want %v", iParsed.ResolvedAt, iOrig.ResolvedAt)
	}

	// Verify metrics aggregates
	if len(parsed.MetricsAggregates) != len(original.MetricsAggregates) {
		t.Fatalf("MetricsAggregates length mismatch: got %d, want %d", len(parsed.MetricsAggregates), len(original.MetricsAggregates))
	}
	aggOrig := original.MetricsAggregates["dev-1"]["temperature"]
	aggParsed := parsed.MetricsAggregates["dev-1"]["temperature"]
	if aggParsed != aggOrig {
		t.Errorf("MetricAggregate mismatch: got %+v, want %+v", aggParsed, aggOrig)
	}
}

// 3. Version Compatibility (older versions)
func TestVersionCompatibility(t *testing.T) {
	// Older version without report_version and missing maps/slices
	legacyJSON := `{
		"session_id": "legacy-session-id",
		"generated_at": "2026-06-04T12:00:00Z",
		"workspace_id": "legacy-workspace-id",
		"session_summary": "Legacy summary"
	}`

	parsed, err := session.ParseArtifactPayload([]byte(legacyJSON))
	if err != nil {
		t.Fatalf("failed to parse legacy JSON: %v", err)
	}

	// Verify default versions and maps were initialized
	if parsed.ReportVersion != "1.0" {
		t.Errorf("expected ReportVersion to default to '1.0', got %q", parsed.ReportVersion)
	}
	if parsed.DeviceSummaries == nil {
		t.Error("expected DeviceSummaries map to be initialized, got nil")
	}
	if parsed.IncidentsArchive == nil {
		t.Error("expected IncidentsArchive slice to be initialized, got nil")
	}
	if parsed.MetricsAggregates == nil {
		t.Error("expected MetricsAggregates map to be initialized, got nil")
	}
}

// 4. Forward Compatibility (future/unknown fields)
func TestForwardCompatibility(t *testing.T) {
	futureJSON := `{
		"session_id": "future-session-id",
		"generated_at": "2026-06-04T12:00:00Z",
		"report_version": "4.0",
		"workspace_id": "future-workspace-id",
		"session_summary": "Future summary",
		"extra_new_field": "some-value",
		"nested_new_object": {
			"sub_key": 1234
		}
	}`

	parsed, err := session.ParseArtifactPayload([]byte(futureJSON))
	if err != nil {
		t.Fatalf("failed to parse future JSON: %v", err)
	}

	// Known fields should survive
	if parsed.SessionID != "future-session-id" {
		t.Errorf("expected SessionID 'future-session-id', got %q", parsed.SessionID)
	}
	if parsed.ReportVersion != "4.0" {
		t.Errorf("expected ReportVersion '4.0', got %q", parsed.ReportVersion)
	}
}

// 5. Golden File Tests
func TestGoldenFile(t *testing.T) {
	goldenPath := filepath.Join("testdata", "session_artifact_golden.json")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("failed to read golden file: %v", err)
	}

	parsed, err := session.ParseArtifactPayload(data)
	if err != nil {
		t.Fatalf("failed to parse golden file: %v", err)
	}

	// Verify golden file values match what we expect
	if parsed.SessionID != "86dd16fb-c4c0-4ffb-8138-a4c7db5fdfaa" {
		t.Errorf("mismatch on golden SessionID: %v", parsed.SessionID)
	}
	if parsed.ReportVersion != "3.0" {
		t.Errorf("mismatch on golden ReportVersion: %v", parsed.ReportVersion)
	}
	if parsed.WorkspaceID != "workspace-123" {
		t.Errorf("mismatch on golden WorkspaceID: %v", parsed.WorkspaceID)
	}

	d2, ok := parsed.DeviceSummaries["device-2"]
	if !ok {
		t.Fatalf("expected device-2 summary in golden artifact")
	}
	if d2.SampleCount != 60 || d2.WarningIncidentsCount != 1 || !d2.ActiveAtEnd {
		t.Errorf("mismatch on device-2 details in golden artifact: %+v", d2)
	}

	if len(parsed.IncidentsArchive) != 1 {
		t.Fatalf("expected 1 incident in golden archive, got %d", len(parsed.IncidentsArchive))
	}
	inc := parsed.IncidentsArchive[0]
	if inc.DeviceID != "device-2" || inc.Metric != "temperature" || inc.IncidentType != "high_temperature" || inc.Severity != "warning" {
		t.Errorf("mismatch on incident details in golden artifact: %+v", inc)
	}

	batteryAgg, ok := parsed.MetricsAggregates["device-1"]["battery"]
	if !ok {
		t.Fatalf("expected battery aggregate for device-1 in golden artifact")
	}
	if batteryAgg.Count != 60 || batteryAgg.Min != 85.0 || batteryAgg.Max != 90.0 || batteryAgg.Average != 87.5 || batteryAgg.Variance != 1.2 {
		t.Errorf("mismatch on battery aggregate: %+v", batteryAgg)
	}
}

// 6. Frontend Compatibility Tag Reflection Verification
func TestFrontendCompatibilityJSONTags(t *testing.T) {
	verifyJSONTags(t, reflect.TypeOf(session.SessionArtifactPayload{}), map[string]string{
		"SessionID":         "session_id",
		"GeneratedAt":       "generated_at",
		"ReportVersion":     "report_version",
		"WorkspaceID":       "workspace_id",
		"SessionSummary":    "session_summary",
		"DeviceSummaries":   "device_summaries",
		"IncidentsArchive":  "incidents_archive",
		"MetricsAggregates": "metrics_aggregates",
	})

	verifyJSONTags(t, reflect.TypeOf(session.DeviceSummaryArtifact{}), map[string]string{
		"DeviceID":              "device_id",
		"FirstSeen":             "first_seen",
		"LastSeen":              "last_seen",
		"SampleCount":           "sample_count",
		"WarningIncidentsCount":  "warning_incidents_count",
		"CriticalIncidentsCount": "critical_incidents_count",
		"ActiveAtEnd":           "active_at_end",
	})

	verifyJSONTags(t, reflect.TypeOf(session.ArtifactIncident{}), map[string]string{
		"DeviceID":     "device_id",
		"Metric":       "metric",
		"IncidentType": "incident_type",
		"Severity":     "severity",
		"StartTime":    "start_time",
		"ResolvedAt":   "resolved_at,omitempty",
		"Occurrences":  "occurrences",
		"PeakScore":    "peak_score",
		"Summary":      "summary",
	})

	verifyJSONTags(t, reflect.TypeOf(session.MetricAggregate{}), map[string]string{
		"Count":    "count",
		"Min":      "min",
		"Max":      "max",
		"Average":  "average",
		"Variance": "variance",
	})
}

func verifyJSONTags(t *testing.T, structType reflect.Type, expected map[string]string) {
	t.Helper()
	for fieldName, expectedTag := range expected {
		field, found := structType.FieldByName(fieldName)
		if !found {
			t.Errorf("struct %s missing expected field %s", structType.Name(), fieldName)
			continue
		}
		actualTag := field.Tag.Get("json")
		if actualTag != expectedTag {
			t.Errorf("struct %s field %s tag mismatch: got %q, want %q", structType.Name(), fieldName, actualTag, expectedTag)
		}
	}
}

