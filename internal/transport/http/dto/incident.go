package dto

import (
	"time"

	"github.com/vishalss1/argus/internal/domain/incident"
)

type IncidentResponse struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	Summary    string     `json:"summary"`
	Severity   string     `json:"severity"`
	Status     string     `json:"status"`
	DeviceIDs  []string   `json:"device_ids"`
	EventIDs   []string   `json:"event_ids"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func ToIncidentResponse(inc incident.Incident) IncidentResponse {
	return IncidentResponse{
		ID:         inc.ID,
		Title:      inc.Title,
		Summary:    inc.Summary,
		Severity:   inc.Severity,
		Status:     string(inc.Status),
		DeviceIDs:  inc.DeviceIDs,
		EventIDs:   inc.EventIDs,
		StartedAt:  inc.StartedAt,
		ResolvedAt: inc.ResolvedAt,
		CreatedAt:  inc.CreatedAt,
		UpdatedAt:  inc.UpdatedAt,
	}
}

func ToIncidentResponses(incidents []incident.Incident) []IncidentResponse {
	res := make([]IncidentResponse, len(incidents))
	for i, inc := range incidents {
		res[i] = ToIncidentResponse(inc)
	}
	return res
}
