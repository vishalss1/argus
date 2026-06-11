package dto

import (
	"encoding/json"
	"time"

	pb "github.com/vishalss1/argus/shared/proto/telemetry"
)

type OperationalMemoryResponse struct {
	ID        string          `json:"id"`
	DeviceID  *string         `json:"device_id,omitempty"`
	Type      string          `json:"type"`
	Summary   string          `json:"summary"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
	CreatedAt time.Time       `json:"created_at"`
}

func ToOperationalMemoryResponse(mem *pb.DeviceHistoryEntry) OperationalMemoryResponse {
	var devID *string
	if mem.DeviceId != "" {
		devID = &mem.DeviceId
	}

	var data json.RawMessage
	if mem.DataJson != "" {
		data = json.RawMessage(mem.DataJson)
	} else {
		data = json.RawMessage(`{}`)
	}

	return OperationalMemoryResponse{
		ID:        mem.Id,
		DeviceID:  devID,
		Type:      mem.Type,
		Summary:   mem.Summary,
		Data:      data,
		Timestamp: mem.Timestamp.AsTime(),
		CreatedAt: mem.CreatedAt.AsTime(),
	}
}

func ToOperationalMemoryResponses(memories []*pb.DeviceHistoryEntry) []OperationalMemoryResponse {
	res := make([]OperationalMemoryResponse, len(memories))
	for i, mem := range memories {
		res[i] = ToOperationalMemoryResponse(mem)
	}
	return res
}
