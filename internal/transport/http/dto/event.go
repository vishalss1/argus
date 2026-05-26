package dto

import (
	"encoding/json"
	"time"

	"github.com/vishalss1/argus/internal/domain/event"
)

type EventResponse struct {
	ID              string          `json:"id"`
	DeviceID        string          `json:"device_id"`
	Type            string          `json:"type"`
	Severity        string          `json:"severity"`
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	Source          string          `json:"source"`
	ConfidenceScore float64         `json:"confidence_score"`
	Metadata        json.RawMessage `json:"metadata"`
	CreatedAt       time.Time       `json:"created_at"`
}

func ToEventResponse(ev event.Event) EventResponse {
	return EventResponse{
		ID:              ev.ID,
		DeviceID:        ev.DeviceID,
		Type:            ev.Type,
		Severity:        string(ev.Severity),
		Title:           ev.Title,
		Summary:         ev.Summary,
		Source:          ev.Source,
		ConfidenceScore: ev.ConfidenceScore,
		Metadata:        ev.Metadata,
		CreatedAt:       ev.CreatedAt,
	}
}

func ToEventResponses(events []event.Event) []EventResponse {
	res := make([]EventResponse, len(events))
	for i, ev := range events {
		res[i] = ToEventResponse(ev)
	}
	return res
}
