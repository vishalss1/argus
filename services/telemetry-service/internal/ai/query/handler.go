package query

import (
	"context"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
)

type responseSource int

const (
	sourceDeterministic responseSource = iota
	sourceRAG
	sourceLLM
)

func (s responseSource) String() string {
	switch s {
	case sourceDeterministic:
		return "deterministic"
	case sourceRAG:
		return "rag"
	case sourceLLM:
		return "llm"
	default:
		return "unknown"
	}
}

type QueryRequest struct {
	Query          string
	Intent         operations.Intent
	FleetMetric    FleetMetric
	TargetDeviceID string
	WorkspaceID    string
}


type QueryHandler interface {
	Handle(ctx context.Context, req QueryRequest) (*Response, error)
}
