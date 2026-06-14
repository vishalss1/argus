package operations

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type RootCauseAnalyzer struct{}

func NewRootCauseAnalyzer() *RootCauseAnalyzer {
	return &RootCauseAnalyzer{}
}

func (a *RootCauseAnalyzer) Analyze(snapshot Snapshot) RootCauseAnalysis {
	incidents := append([]Incident(nil), snapshot.IncidentHistory...)
	sort.Slice(incidents, func(i, j int) bool { return incidents[i].StartTime.Before(incidents[j].StartTime) })

	result := RootCauseAnalysis{
		Confidence:         35,
		PrimaryCause:       "No single failure cause is confirmed; current device state and telemetry should be inspected.",
		SupportingEvidence: make([]string, 0),
		AlternativeCauses:  []string{"application fault", "connectivity degradation", "telemetry producer issue"},
		RecommendedActions: []string{"Inspect device and application logs", "Verify telemetry and connectivity", "Continue monitoring for a reproducible pattern"},
	}
	if strings.ToLower(snapshot.Device.Status) != "online" {
		result.Confidence = 75
		result.PrimaryCause = "Connectivity or device availability failure is the leading cause."
		result.SupportingEvidence = append(result.SupportingEvidence, "Device is currently offline")
		result.RecommendedActions = []string{"Verify power and network connectivity", "Check heartbeat delivery", "Inspect reboot and connectivity logs"}
		result.AlternativeCauses = []string{"device reboot", "network path failure", "heartbeat producer failure"}
	}
	if len(incidents) > 0 {
		first := incidents[0]
		result.SupportingEvidence = append(result.SupportingEvidence, fmt.Sprintf("%s %s was the first observed incident", first.Metric, first.IncidentType))
		if first.IncidentType == "numeric_stuck" && strings.ToLower(snapshot.Device.Status) == "online" {
			result.Confidence = 84
			result.PrimaryCause = "The telemetry producer or application task appears stalled while the device remains online."
			result.AlternativeCauses = []string{"application deadlock", "sensor reporting freeze", "metric producer defect"}
		} else if first.IncidentType == "numeric_spike" || first.IncidentType == "numeric_drop" {
			result.Confidence = 72
			result.PrimaryCause = fmt.Sprintf("%s deviated first and is the leading indicator of the failure.", first.Metric)
		}
		result.SupportingEvidence = append(result.SupportingEvidence, fmt.Sprintf("Incident occurred %d times with peak score %.2f", first.Occurrences, first.PeakScore))
		remediation := NewRemediationEngine().ForIncident(first)
		result.RecommendedActions = remediation.Actions
	}
	if strings.ToLower(snapshot.Device.Status) == "online" {
		result.SupportingEvidence = append(result.SupportingEvidence, "Device remains online, reducing the likelihood of a network outage")
	}
	if snapshot.TelemetryRecordedAt != nil {
		result.SupportingEvidence = append(result.SupportingEvidence, fmt.Sprintf("Latest telemetry was recorded %s ago", time.Since(*snapshot.TelemetryRecordedAt).Round(time.Second)))
	}
	if samples := len(snapshot.TelemetryWindows["previous1h"]); samples > 0 {
		result.SupportingEvidence = append(result.SupportingEvidence, fmt.Sprintf("%d telemetry samples were analyzed from the previous hour", samples))
	}
	if len(snapshot.IncidentHistory) > 1 {
		result.SupportingEvidence = append(result.SupportingEvidence, fmt.Sprintf("%d incidents are present in the recent history", len(snapshot.IncidentHistory)))
		result.Confidence += 5
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}
	result.SupportingEvidence = unique(result.SupportingEvidence, 7)
	return result
}
