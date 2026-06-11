package dto

import "encoding/json"

type UpdateShadowStateRequest struct {
	State json.RawMessage `json:"state" validate:"required" swaggertype:"object"`
}

