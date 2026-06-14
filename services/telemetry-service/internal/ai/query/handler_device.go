package query

import (
	"context"
	"fmt"
	"log"

	"github.com/vishalss1/argus/telemetry/internal/ai/operations"
)

type DeviceHealthHandler struct {
	snapshotBuilder   *SnapshotBuilder
	summaryAnalyzer   *operations.DeviceSummaryAnalyzer
	rootCauseAnalyzer *operations.RootCauseAnalyzer
	remediationEngine *operations.RemediationEngine
	engine            *Engine // temporarily reference Engine to call reasonOverSnapshot
}

func NewDeviceHealthHandler(
	snapshotBuilder *SnapshotBuilder,
	engine *Engine,
) *DeviceHealthHandler {
	return &DeviceHealthHandler{
		snapshotBuilder:   snapshotBuilder,
		summaryAnalyzer:   operations.NewDeviceSummaryAnalyzer(),
		rootCauseAnalyzer: operations.NewRootCauseAnalyzer(),
		remediationEngine: operations.NewRemediationEngine(),
		engine:            engine,
	}
}

func (h *DeviceHealthHandler) Handle(ctx context.Context, req QueryRequest) (*Response, error) {
	snapshot, err := h.snapshotBuilder.Build(ctx, req.TargetDeviceID)
	if err != nil {
		return nil, fmt.Errorf("build operational context: %w", err)
	}

	summary := h.summaryAnalyzer.Analyze(*snapshot)
	rca := h.rootCauseAnalyzer.Analyze(*snapshot)
	remediations := h.remediationEngine.Analyze(snapshot.IncidentHistory)
	snapshot.GeneratedAnalysis = map[string]any{
		"deviceSummary": summary,
		"rootCause":     rca,
		"remediations":  remediations,
	}

	response := &Response{
		Intent:             req.Intent,
		DeviceSummary:      &summary,
		OperationalContext: snapshot,
		RelatedDevices:     h.engine.relatedDevices(ctx, *snapshot),
		source:             sourceDeterministic,
	}

	switch req.Intent {
	case operations.IntentRootCauseAnalysis:
		response.Summary = rca.PrimaryCause
		response.Evidence = rca.SupportingEvidence
		response.SuggestedActions = rca.RecommendedActions
		response.RootCauseAnalysis = &rca
	case operations.IntentRemediation:
		response.Summary = remediationSummary(snapshot.Device.Name, remediations)
		response.Evidence = rca.SupportingEvidence
		response.SuggestedActions = flattenActions(remediations, summary.Recommendations)
		response.RootCauseAnalysis = &rca
		response.Remediations = remediations
	default:
		response.Summary = fmt.Sprintf("%s is %s with a health score of %d/100.", snapshot.Device.Name, summary.Severity, summary.HealthScore)
		response.Evidence = summary.KeyFindings
		response.SuggestedActions = summary.Recommendations
	}

	// Optional enrichment via LLM
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
