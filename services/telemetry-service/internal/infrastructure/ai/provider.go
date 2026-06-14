package ai

type ReasoningResponse struct {
	Summary          string   `json:"summary"`
	Confidence       float64  `json:"confidence"`
	Evidence         []string `json:"evidence"`
	SuggestedActions []string `json:"suggested_actions"`
}
