package dto

type CreateRuleRequest struct {
	Name      string  `json:"name" validate:"required"`
	Metric    string  `json:"metric" validate:"required"`
	Operator  string  `json:"operator" validate:"required"`
	Threshold float64 `json:"threshold"`
	Enabled   *bool   `json:"enabled,omitempty"`
}

type UpdateRuleRequest struct {
	Name      *string  `json:"name,omitempty"`
	Metric    *string  `json:"metric,omitempty"`
	Operator  *string  `json:"operator,omitempty"`
	Threshold *float64 `json:"threshold,omitempty"`
	Enabled   *bool    `json:"enabled,omitempty"`
}
