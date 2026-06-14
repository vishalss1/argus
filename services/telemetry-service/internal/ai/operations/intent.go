package operations

import "strings"

func ClassifyIntent(query string) Intent {
	q := strings.ToLower(strings.TrimSpace(query))
	switch {
	case containsAny(q, "how do i fix", "how to fix", "remediate", "what should i do", "do next", "recommended action", "resolve this"):
		return IntentRemediation
	case containsAny(q, "why did", "root cause", "what caused", "what is wrong", "what's wrong", "happened before the incident", "likely cause"):
		return IntentRootCauseAnalysis
	case containsAny(q, "compare", "similar pattern", "similar devices", "related devices"):
		return IntentDeviceComparison
	case containsAny(q, "how many devices", "devices online", "devices active", "active devices", "online devices", "connected devices", "any incidents", "active incidents", "worst severity", "how many warnings", "how many critical", "critical alerts", "fleet", "all devices", "happened recently", "incidents today", "recent incidents", "summarize fleet", "fleet health", "summarize fleet health"):
		return IntentFleetSummary
	case containsAny(q, "summarize", "summary", "how is", "health", "doing", "status"):
		return IntentDeviceSummary
	case containsAny(q, "incident", "warning", "alert", "anomaly", "has this happened before"):
		return IntentIncidentLookup
	default:
		return IntentUnknown
	}
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}
