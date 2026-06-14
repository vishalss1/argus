package query

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/vishalss1/argus/shared/common"
	"github.com/vishalss1/argus/telemetry/internal/domain/fleet"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/ai"
)

type FleetSummaryHandler struct {
	fleetService fleet.Service
	aiProvider   *ai.GroqProvider
	logger       *zap.Logger
}

func NewFleetSummaryHandler(
	fleetService fleet.Service,
	aiProvider *ai.GroqProvider,
	logger *zap.Logger,
) *FleetSummaryHandler {
	return &FleetSummaryHandler{
		fleetService: fleetService,
		aiProvider:   aiProvider,
		logger:       logger,
	}
}

func (h *FleetSummaryHandler) Handle(ctx context.Context, req QueryRequest) (*Response, error) {
	reqID, _ := common.GetRequestID(ctx)
	start := time.Now()

	stats, err := h.fleetService.GetStats(ctx, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("fleet stats: %w", err)
	}

	h.logger.Info("[FLEET SERVICE]",
		zap.String("request_id", reqID),
		zap.Duration("latency", time.Since(start)),
	)

	// Deterministic answers — no LLM
	switch req.FleetMetric {
	case FleetMetricOnlineDevices:
		return &Response{
			Summary: fmt.Sprintf("There are currently %d devices online.", stats.OnlineDevices),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	case FleetMetricOfflineDevices:
		offline := stats.TotalDevices - stats.OnlineDevices
		if offline < 0 {
			offline = 0
		}
		return &Response{
			Summary: fmt.Sprintf("There are currently %d devices offline.", offline),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	case FleetMetricActiveIncidents:
		return &Response{
			Summary: fmt.Sprintf("There are %d active incidents across the fleet.", stats.ActiveIncidents),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	case FleetMetricWorstSeverity:
		return &Response{
			Summary: fmt.Sprintf("The worst active severity is %s.", stats.WorstSeverity),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	case FleetMetricWarningCount:
		return &Response{
			Summary: fmt.Sprintf("There are %d warning-level incidents active.", stats.WarningIncidents),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	case FleetMetricCriticalCount:
		return &Response{
			Summary: fmt.Sprintf("There are %d critical-level incidents active.", stats.CriticalIncidents),
			Intent:  req.Intent,
			source:  sourceDeterministic,
		}, nil
	}

	// Open-ended fleet question → LLM
	return h.generateFleetSummary(ctx, req, stats, reqID)
}

func (h *FleetSummaryHandler) generateFleetSummary(ctx context.Context, req QueryRequest, stats *fleet.Stats, reqID string) (*Response, error) {
	data, _ := json.Marshal(stats)
	systemPrompt := `You are ARGUS AI. Answer the user's question using the provided live fleet statistics. Keep it concise, professional, and direct. Output JSON with summary, evidence (MUST be a JSON array of strings), and suggested_actions (MUST be a JSON array of strings).`

	llmStart := time.Now()
	reasoning, reasonErr := h.aiProvider.Reason(ctx, systemPrompt, fmt.Sprintf("Query: %s\nLiveStats: %s", req.Query, data))
	if reasonErr != nil {
		h.logger.Error("[LLM FAILURE]",
			zap.String("request_id", reqID),
			zap.Error(reasonErr),
		)
		return nil, fmt.Errorf("fleet summary reasoning failed: %w", reasonErr)
	}

	h.logger.Info("[LLM REQUEST]",
		zap.String("request_id", reqID),
		zap.Duration("latency", time.Since(llmStart)),
	)

	return &Response{
		Intent:           req.Intent,
		Summary:          reasoning.Summary,
		Evidence:         reasoning.Evidence,
		SuggestedActions: reasoning.SuggestedActions,
		source:           sourceLLM,
	}, nil
}
