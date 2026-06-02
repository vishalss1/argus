package session

import (
	"context"
	"encoding/json"
	"time"
)

type AIReport struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	SummaryText string          `json:"summary_text"`
	Metadata    json.RawMessage `json:"metadata"`
	GeneratedAt time.Time       `json:"generated_at"`
}

type AIReportRepository interface {
	CreateAIReport(ctx context.Context, r AIReport) (*AIReport, error)
	GetAIReportBySession(ctx context.Context, sessionID string) (*AIReport, error)
}
