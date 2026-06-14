package query

import (
	"strings"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
)

type FleetMetric int

const (
	FleetMetricNone FleetMetric = iota
	FleetMetricOnlineDevices
	FleetMetricOfflineDevices
	FleetMetricActiveIncidents
	FleetMetricWorstSeverity
	FleetMetricWarningCount
	FleetMetricCriticalCount
)

type QueryPlan struct {
	Intent         operations.Intent
	FleetMetric    FleetMetric
	TargetDeviceID string
}

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) Build(query, preferredDeviceID string) (QueryPlan, error) {
	intent := operations.ClassifyIntent(query)

	plan := QueryPlan{
		Intent:         intent,
		TargetDeviceID: preferredDeviceID,
	}

	if intent == operations.IntentFleetSummary {
		plan.FleetMetric = classifyFleetMetric(query)
	}

	return plan, nil
}

func classifyFleetMetric(query string) FleetMetric {
	q := strings.ToLower(strings.TrimSpace(query))
	
	if strings.Contains(q, "online") || strings.Contains(q, "connected") {
		return FleetMetricOnlineDevices
	}
	if strings.Contains(q, "offline") || strings.Contains(q, "disconnected") {
		return FleetMetricOfflineDevices
	}
	if strings.Contains(q, "worst severity") {
		return FleetMetricWorstSeverity
	}
	if strings.Contains(q, "warnings") {
		return FleetMetricWarningCount
	}
	if strings.Contains(q, "critical alerts") || strings.Contains(q, "critical") {
		return FleetMetricCriticalCount
	}
	if strings.Contains(q, "how many incidents") || strings.Contains(q, "active incidents") || strings.Contains(q, "any incidents") {
		return FleetMetricActiveIncidents
	}

	return FleetMetricNone
}
