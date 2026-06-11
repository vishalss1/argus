package dto

import (
	"encoding/json"
	"time"

	pb "github.com/vishalss1/argus/shared/proto/telemetry"
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

func ToEventResponse(ev *pb.EventResponse) EventResponse {
	var metadata json.RawMessage
	if ev.MetadataJson != "" {
		metadata = json.RawMessage(ev.MetadataJson)
	} else {
		metadata = json.RawMessage(`{}`)
	}

	return EventResponse{
		ID:              ev.Id,
		DeviceID:        ev.DeviceId,
		Type:            ev.Type,
		Severity:        ev.Severity,
		Title:           ev.Title,
		Summary:         ev.Summary,
		Source:          ev.Source,
		ConfidenceScore: ev.ConfidenceScore,
		Metadata:        metadata,
		CreatedAt:       ev.CreatedAt.AsTime(),
	}
}

func ToEventResponses(events []*pb.EventResponse) []EventResponse {
	res := make([]EventResponse, len(events))
	for i, ev := range events {
		res[i] = ToEventResponse(ev)
	}
	return res
}
