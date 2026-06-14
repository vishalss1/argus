package query

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"

	"github.com/vishalss1/argus/shared/common"
	ctxdomain "github.com/vishalss1/argus/telemetry/internal/domain/context"
	"github.com/vishalss1/argus/telemetry/internal/domain/event"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/ai"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/telemetry/internal/infrastructure/postgres"
)

type HistoricalAnalysisHandler struct {
	embeddingProvider   *embedding.LocalProvider
	vectorStore         *postgres.VectorStore
	eventRepo           *postgres.EventRepository
	contextRepo         *postgres.ContextRepository
	aiProvider          *ai.GroqProvider
	logger              *zap.Logger
	similarityThreshold float32
}

func NewHistoricalAnalysisHandler(
	embeddingProvider *embedding.LocalProvider,
	vectorStore *postgres.VectorStore,
	eventRepo *postgres.EventRepository,
	contextRepo *postgres.ContextRepository,
	aiProvider *ai.GroqProvider,
	logger *zap.Logger,
	similarityThreshold float32,
) *HistoricalAnalysisHandler {
	return &HistoricalAnalysisHandler{
		embeddingProvider:   embeddingProvider,
		vectorStore:         vectorStore,
		eventRepo:           eventRepo,
		contextRepo:         contextRepo,
		aiProvider:          aiProvider,
		logger:              logger,
		similarityThreshold: similarityThreshold,
	}
}

func (h *HistoricalAnalysisHandler) Handle(ctx context.Context, req QueryRequest) (*Response, error) {
	reqID, _ := common.GetRequestID(ctx)
	retrievalStart := time.Now()

	retrieved, err := h.RetrieveContext(ctx, req.Query)
	if err != nil {
		h.logger.Error("[QUERY ENGINE] semantic retrieval failed",
			zap.String("request_id", reqID),
			zap.Error(err),
		)
		retrieved = &Context{}
	}

	h.logger.Info("[RAG RETRIEVAL]",
		zap.String("request_id", reqID),
		zap.Int("events", len(retrieved.Events)),
		zap.Int("memories", len(retrieved.Memories)),
		zap.Duration("latency", time.Since(retrievalStart)),
	)

	// Short-circuit if nothing relevant is found
	if len(retrieved.Events) == 0 && len(retrieved.Memories) == 0 {
		return &Response{
			Summary: "Unable to find operational information relevant to the query.",
			Intent:  req.Intent,
			source:  sourceRAG,
		}, nil
	}

	data, _ := json.Marshal(retrieved)
	systemPrompt := `You are ARGUS AI. Answer from the supplied operational events and memory. If exact incidents are absent, provide useful fleet observations and recommended next checks. Never answer "No matching incidents found". Output JSON with summary, evidence, suggested_actions.`

	llmStart := time.Now()
	reasoning, reasonErr := h.aiProvider.Reason(ctx, systemPrompt, fmt.Sprintf("Intent: %s\nQuery: %s\nContext: %s", req.Intent, req.Query, data))
	if reasonErr != nil {
		h.logger.Error("[LLM FAILURE]",
			zap.String("request_id", reqID),
			zap.Error(reasonErr),
		)
		return nil, fmt.Errorf("historical analysis reasoning failed: %w", reasonErr)
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
		source:           sourceRAG,
	}, nil
}

type Context struct {
	Events   []event.Event                 `json:"events"`
	Memories []ctxdomain.OperationalMemory `json:"memories"`
}

func (h *HistoricalAnalysisHandler) RetrieveContext(ctx context.Context, queryString string) (*Context, error) {
	retrieved := &Context{}
	seen := make(map[string]bool)
	uuidRegex := regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	for _, id := range uuidRegex.FindAllString(queryString, -1) {
		if ev, err := h.eventRepo.GetByID(ctx, id); err == nil {
			retrieved.Events = append(retrieved.Events, *ev)
			seen[ev.ID] = true
		}
	}
	vector, err := h.embeddingProvider.Embed(ctx, queryString)
	if err != nil {
		return retrieved, err
	}
	if results, err := h.vectorStore.Search(ctx, "events", vector, 5); err == nil {
		for _, result := range results {
			if result.Score >= h.similarityThreshold && !seen[result.ID] {
				if ev, getErr := h.eventRepo.GetByID(ctx, result.ID); getErr == nil {
					retrieved.Events = append(retrieved.Events, *ev)
					seen[result.ID] = true
				}
			}
		}
	}
	if results, err := h.vectorStore.Search(ctx, "operational_memory", vector, 5); err == nil {
		for _, result := range results {
			if result.Score >= h.similarityThreshold && !seen[result.ID] {
				if memory, getErr := h.contextRepo.GetByID(ctx, result.ID); getErr == nil {
					retrieved.Memories = append(retrieved.Memories, *memory)
					seen[result.ID] = true
				}
			}
		}
	}
	return retrieved, nil
}
