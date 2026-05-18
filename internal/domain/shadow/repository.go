package shadow

import (
	"context"
	"encoding/json"
)

type Repository interface {
	Get(ctx context.Context, deviceID string) (*Shadow, error)
	UpdateDesired(ctx context.Context, deviceID string, state json.RawMessage) (*Shadow, error)
	UpdateReported(ctx context.Context, deviceID string, state json.RawMessage) (*Shadow, error)
}
