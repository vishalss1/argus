package operations

import "fmt"

type RemediationEngine struct{}

func NewRemediationEngine() *RemediationEngine {
	return &RemediationEngine{}
}

func (e *RemediationEngine) ForIncident(incident Incident) Remediation {
	pattern := incident.Metric + "." + incident.IncidentType
	remediation := Remediation{
		Pattern:  pattern,
		Metric:   incident.Metric,
		DeviceID: incident.DeviceID,
		Severity: incident.Severity,
	}
	switch incident.IncidentType {
	case "numeric_stuck":
		remediation.Reasoning = fmt.Sprintf("%s stopped changing while the analyzer continued receiving samples, which usually indicates a stalled producer or blocked application path.", incident.Metric)
		remediation.PossibleCauses = []string{"telemetry producer freeze", "application deadlock", "sensor or metric reporting freeze"}
		remediation.Actions = []string{"Restart the application telemetry task", "Inspect the metric producer and application logs", "Verify metric updates resume after restart", "Review firmware for deadlocks or blocked allocation paths"}
	case "numeric_spike":
		remediation.Reasoning = fmt.Sprintf("%s rose sharply outside its observed baseline.", incident.Metric)
		remediation.PossibleCauses = []string{"resource saturation", "unexpected workload", "sensor fault"}
		remediation.Actions = []string{"Inspect workload and resource usage around the first deviation", "Check firmware and application logs", "Validate the sensor or metric source", "Reduce load or restart the affected service if the value remains unsafe"}
	case "numeric_drop":
		remediation.Reasoning = fmt.Sprintf("%s fell sharply below its observed baseline.", incident.Metric)
		remediation.PossibleCauses = []string{"resource depletion", "component failure", "telemetry corruption"}
		remediation.Actions = []string{"Inspect the first low sample and surrounding logs", "Verify resource availability and dependent components", "Validate the telemetry source", "Restart the affected service if the metric does not recover"}
	case "binary_toggle", "categorical_change":
		remediation.Reasoning = fmt.Sprintf("%s changed state unexpectedly.", incident.Metric)
		remediation.PossibleCauses = []string{"configuration change", "component restart", "intermittent connectivity"}
		remediation.Actions = []string{"Confirm whether the state change was expected", "Review configuration and deployment history", "Inspect connectivity and restart logs", "Monitor for repeated state changes"}
	default:
		remediation.Reasoning = "The incident requires validation against telemetry, connectivity, and firmware logs."
		remediation.PossibleCauses = []string{"application fault", "connectivity degradation", "telemetry producer issue"}
		remediation.Actions = []string{"Inspect telemetry and firmware logs", "Verify device connectivity", "Restart the affected application component", "Continue monitoring for recurrence"}
	}
	return remediation
}

func (e *RemediationEngine) Analyze(incidents []Incident) []Remediation {
	result := make([]Remediation, 0)
	seen := make(map[string]bool)
	for _, incident := range incidents {
		if incident.Status != "open" {
			continue
		}
		item := e.ForIncident(incident)
		if !seen[item.Pattern] {
			seen[item.Pattern] = true
			result = append(result, item)
		}
	}
	return result
}
