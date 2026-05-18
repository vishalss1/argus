package dto

import "encoding/json"

type SendCommandRequest struct {
	Type    string          `json:"type" validate:"required"`
	Payload json.RawMessage `json:"payload,omitempty" swaggertype:"object"`
}

type CommandResultRequest struct {
	Message string `json:"message,omitempty"`
}
