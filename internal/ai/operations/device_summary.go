package operations

import (
	"fmt"
	"strings"
	"time"
)

type DeviceSummaryAnalyzer struct{}

func NewDeviceSummaryAnalyzer() *DeviceSummaryAnalyzer {
	return &DeviceSummaryAnalyzer{}
}

func (a *DeviceSummaryAnalyzer) Analyze(snapshot Snapshot) DeviceSummary {
	now := time.Now().UTC()
	open := 0
	last24h := 0
	last7d := 0
	severity := "healthy"
	findings := make([]string, 0)

	for _, incident := range snapshot.IncidentHistory {
		if incident.Status == "open" {
			open++
			if incident.Severity == "critical" || (incident.Severity == "warning" && severity == "healthy") {
				severity = incident.Severity
			}
		}
		if incident.StartTime.After(now.Add(-24 * time.Hour)) {
			last24h++
		}
		if incident.StartTime.After(now.Add(-7 * 24 * time.Hour)) {
			last7d++
		}
	}
	for _, item := range snapshot.AnomalyHistory {
		if item.Type != "incident" {
			continue
		}
		if item.Timestamp.After(now.Add(-24 * time.Hour)) {
			last24h++
		}
		if item.Timestamp.After(now.Add(-7 * 24 * time.Hour)) {
			last7d++
		}
	}

	status := strings.ToLower(snapshot.Device.Status)
	score := 100
	if status != "online" {
		score -= 35
		severity = "critical"
		findings = append(findings, "Device is offline or not reporting a healthy presence state")
	}
	for _, incident := range snapshot.IncidentHistory {
		if incident.Status != "open" {
			continue
		}
		if incident.Severity == "critical" {
			score -= 22
		} else {
			score -= 10
		}
		findings = append(findings, fmt.Sprintf("%s %s is active (%d occurrences)", incident.Metric, incident.IncidentType, incident.Occurrences))
	}
	for _, trend := range snapshot.TelemetryTrends {
		if trend.Anomalous {
			score -= 5
			findings = append(findings, fmt.Sprintf("%s is outside its observed operating pattern", trend.Metric))
		}
	}
	if snapshot.TelemetryRecordedAt == nil {
		score -= 15
		findings = append(findings, "No recent telemetry sample is available")
	} else if age := now.Sub(*snapshot.TelemetryRecordedAt); age > 10*time.Minute {
		score -= 15
		findings = append(findings, fmt.Sprintf("Latest telemetry is stale by %s", age.Round(time.Minute)))
	}
	if len(findings) == 0 {
		findings = append(findings, "No active anomalies detected", "Device telemetry and connectivity appear stable")
	}
	if score < 0 {
		score = 0
	}

	return DeviceSummary{
		DeviceID:         snapshot.Device.ID,
		DeviceName:       snapshot.Device.Name,
		DeviceStatus:     status,
		Severity:         severity,
		OpenIncidents:    open,
		RecentIncidents:  last24h,
		IncidentsLast24h: last24h,
		IncidentsLast7d:  last7d,
		HealthScore:      score,
		KeyFindings:      unique(findings, 6),
		Recommendations:  summaryRecommendations(status, snapshot.IncidentHistory),
	}
}

func summaryRecommendations(status string, incidents []Incident) []string {
	actions := make([]string, 0)
	if status != "online" {
		actions = append(actions, "Verify power, network reachability, and device heartbeat delivery")
	}
	for _, incident := range incidents {
		if incident.Status == "open" {
			actions = append(actions, NewRemediationEngine().ForIncident(incident).Actions...)
		}
	}
	if len(actions) == 0 {
		actions = append(actions, "Continue monitoring current telemetry and connectivity")
	}
	return unique(actions, 5)
}

func unique(values []string, limit int) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
		if len(result) == limit {
			break
		}
	}
	return result
}
