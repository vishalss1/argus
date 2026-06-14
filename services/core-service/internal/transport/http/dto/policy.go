package dto

import (
	"time"

	"github.com/vishalss1/argus/core/internal/domain/policy"
)

type ExecutionRecordResponse struct {
	ID          string     `json:"id"`
	Action      string     `json:"action"`
	DeviceID    string     `json:"device_id"`
	IncidentID  *string    `json:"incident_id,omitempty"`
	Status      string     `json:"status"`
	SuggestedBy string     `json:"suggested_by"`
	ApprovedBy  *string    `json:"approved_by,omitempty"`
	Metadata    string     `json:"metadata"`
	CreatedAt   time.Time  `json:"created_at"`
}

func ToExecutionRecordResponse(r policy.ExecutionRecord) ExecutionRecordResponse {
	return ExecutionRecordResponse{
		ID:          r.ID,
		Action:      string(r.Action),
		DeviceID:    r.DeviceID,
		IncidentID:  r.IncidentID,
		Status:      r.Status,
		SuggestedBy: r.SuggestedBy,
		ApprovedBy:  r.ApprovedBy,
		Metadata:    r.Metadata,
		CreatedAt:   r.CreatedAt,
	}
}

func ToExecutionRecordResponses(records []policy.ExecutionRecord) []ExecutionRecordResponse {
	res := make([]ExecutionRecordResponse, len(records))
	for i, r := range records {
		res[i] = ToExecutionRecordResponse(r)
	}
	return res
}

type ApproveActionRequest struct {
	ApprovedBy string `json:"approved_by" validate:"required"`
}

