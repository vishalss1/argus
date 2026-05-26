package dto

import (
	"encoding/json"
	"time"

	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
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

func ToOperationalMemoryResponse(mem ctxdomain.OperationalMemory) OperationalMemoryResponse {
	return OperationalMemoryResponse{
		ID:        mem.ID,
		DeviceID:  mem.DeviceID,
		Type:      string(mem.Type),
		Summary:   mem.Summary,
		Data:      mem.Data,
		Timestamp: mem.Timestamp,
		CreatedAt: mem.CreatedAt,
	}
}

func ToOperationalMemoryResponses(memories []ctxdomain.OperationalMemory) []OperationalMemoryResponse {
	res := make([]OperationalMemoryResponse, len(memories))
	for i, mem := range memories {
		res[i] = ToOperationalMemoryResponse(mem)
	}
	return res
}
