package query

import (
	"context"
	"fmt"
	"log"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
)

type IncidentHandler struct {
	snapshotBuilder *SnapshotBuilder
	summaryAnalyzer *operations.DeviceSummaryAnalyzer
	engine          *Engine
}

func NewIncidentHandler(snapshotBuilder *SnapshotBuilder, engine *Engine) *IncidentHandler {
	return &IncidentHandler{
		snapshotBuilder: snapshotBuilder,
		summaryAnalyzer: operations.NewDeviceSummaryAnalyzer(),
		engine:          engine,
	}
}

func (h *IncidentHandler) Handle(ctx context.Context, req QueryRequest) (*Response, error) {
	snapshot, err := h.snapshotBuilder.Build(ctx, req.TargetDeviceID)
	if err != nil {
		return nil, fmt.Errorf("build operational context: %w", err)
	}

	summary := h.summaryAnalyzer.Analyze(*snapshot)

	response := &Response{
		Intent:             req.Intent,
		DeviceSummary:      &summary,
		OperationalContext: snapshot,
		RelatedDevices:     h.engine.relatedDevices(ctx, *snapshot),
		source:             sourceDeterministic,
	}

	response.Summary = fmt.Sprintf("%s has %d open and %d recent incidents.", snapshot.Device.Name, summary.OpenIncidents, summary.RecentIncidents)
	response.Evidence = incidentEvidence(snapshot.IncidentHistory)
	response.SuggestedActions = summary.Recommendations
	response.Remediations = h.engine.remediationEngine.Analyze(snapshot.IncidentHistory)

	if enriched, err := h.engine.reasonOverSnapshot(ctx, req.Query, req.Intent, snapshot); err == nil {
		if enriched.Summary != "" {
			response.Summary = enriched.Summary
		}
		if len(enriched.Evidence) > 0 {
			response.Evidence = uniqueStrings(append(response.Evidence, enriched.Evidence...), 8)
		}
		if len(enriched.SuggestedActions) > 0 {
			response.SuggestedActions = uniqueStrings(append(response.SuggestedActions, enriched.SuggestedActions...), 8)
		}
		response.source = sourceLLM
	} else {
		log.Printf("[QUERY ENGINE] structured LLM reasoning unavailable, returning deterministic analysis: %v", err)
	}

	return response, nil
}
