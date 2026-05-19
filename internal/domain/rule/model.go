package rule

import "time"

const (
	OperatorGreaterThan        = ">"
	OperatorGreaterThanOrEqual = ">="
	OperatorLessThan           = "<"
	OperatorLessThanOrEqual    = "<="
	OperatorEqual              = "=="
	OperatorNotEqual           = "!="
)

type Rule struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Metric    string    `json:"metric" db:"metric"`
	Operator  string    `json:"operator" db:"operator"`
	Threshold float64   `json:"threshold" db:"threshold"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Alert struct {
	ID            string    `json:"id" db:"id"`
	RuleID        string    `json:"rule_id" db:"rule_id"`
	DeviceID      string    `json:"device_id" db:"device_id"`
	TelemetryID   string    `json:"telemetry_id" db:"telemetry_id"`
	Metric        string    `json:"metric" db:"metric"`
	Operator      string    `json:"operator" db:"operator"`
	Threshold     float64   `json:"threshold" db:"threshold"`
	ObservedValue float64   `json:"observed_value" db:"observed_value"`
	Message       string    `json:"message" db:"message"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

type CreateInput struct {
	Name      string
	Metric    string
	Operator  string
	Threshold float64
	Enabled   *bool
}

type UpdateInput struct {
	Name      *string
	Metric    *string
	Operator  *string
	Threshold *float64
	Enabled   *bool
}
