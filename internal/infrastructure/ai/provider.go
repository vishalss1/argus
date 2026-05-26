package ai

import "context"

type ReasoningResponse struct {
	Summary          string   `json:"summary"`
	Confidence       float64  `json:"confidence"`
	Evidence         []string `json:"evidence"`
	SuggestedActions []string `json:"suggested_actions"`
}

type Provider interface {
	Reason(ctx context.Context, systemPrompt, userPrompt string) (*ReasoningResponse, error)
}
