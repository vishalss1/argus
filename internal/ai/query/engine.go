package query

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/incident"
	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
)

type Engine struct {
	embeddingProvider embedding.Provider
	aiProvider        ai.Provider
	vectorStore      *postgres.VectorStore
	eventRepo        *postgres.EventRepository
	incidentRepo     *postgres.IncidentRepository
	contextRepo      *postgres.ContextRepository
}

func NewEngine(
	embeddingProvider embedding.Provider,
	aiProvider ai.Provider,
	vectorStore *postgres.VectorStore,
	eventRepo *postgres.EventRepository,
	incidentRepo *postgres.IncidentRepository,
	contextRepo *postgres.ContextRepository,
) *Engine {
	return &Engine{
		embeddingProvider: embeddingProvider,
		aiProvider:        aiProvider,
		vectorStore:      vectorStore,
		eventRepo:        eventRepo,
		incidentRepo:     incidentRepo,
		contextRepo:      contextRepo,
	}
}

func (e *Engine) Ask(ctx context.Context, queryString string) (*ai.ReasoningResponse, error) {
	// 1. Retrieve Context
	retrieved, err := e.RetrieveContext(ctx, queryString)
	if err != nil {
		return nil, fmt.Errorf("retrieve context: %w", err)
	}

	// 2. Format Context for LLM
	contextText := e.formatContext(retrieved)

	// 3. Define System Prompt
	systemPrompt := `You are ARGUS AI, an operational reasoning subsystem for a device fleet.
Your goal is to provide grounded, evidence-based answers to operator queries.
You must ONLY use the provided context. If the answer is not in the context, say you don't know.
Always provide a confidence score and list the evidence used.
Suggest operational actions when appropriate.
Output MUST be a valid JSON object matching the requested schema.`

	userPrompt := fmt.Sprintf("Query: %s\n\nContext:\n%s", queryString, contextText)

	// 4. Call Reasoning Layer
	return e.aiProvider.Reason(ctx, systemPrompt, userPrompt)
}

func (e *Engine) formatContext(c *Context) string {
	var buf bytes.Buffer

	buf.WriteString("--- RELEVANT EVENTS ---\n")
	for _, ev := range c.Events {
		buf.WriteString(fmt.Sprintf("- [%s] %s: %s (%s)\n", ev.CreatedAt.Format(time.RFC3339), ev.Type, ev.Title, ev.Summary))
	}

	buf.WriteString("\n--- RELEVANT INCIDENTS ---\n")
	for _, inc := range c.Incidents {
		buf.WriteString(fmt.Sprintf("- %s: %s (Status: %s, Severity: %s)\n", inc.Title, inc.Summary, inc.Status, inc.Severity))
	}

	buf.WriteString("\n--- OPERATIONAL MEMORY ---\n")
	for _, mem := range c.Memories {
		buf.WriteString(fmt.Sprintf("- %s: %s\n", mem.Type, mem.Summary))
	}

	return buf.String()
}

type Context struct {
	Events    []event.Event
	Incidents []incident.Incident
	Memories  []ctxdomain.OperationalMemory
}

func (e *Engine) RetrieveContext(ctx context.Context, queryString string) (*Context, error) {
	// 1. Generate embedding for the query
	queryVector, err := e.embeddingProvider.Embed(ctx, queryString)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	retrievedContext := &Context{}

	// 2. Search events
	eventResults, err := e.vectorStore.Search(ctx, "events", queryVector, 5)
	if err == nil {
		for _, res := range eventResults {
			ev, err := e.eventRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Events = append(retrievedContext.Events, *ev)
			}
		}
	}

	// 3. Search incidents
	incidentResults, err := e.vectorStore.Search(ctx, "incidents", queryVector, 3)
	if err == nil {
		for _, res := range incidentResults {
			inc, err := e.incidentRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Incidents = append(retrievedContext.Incidents, *inc)
			}
		}
	}

	// 4. Search operational memory
	memoryResults, err := e.vectorStore.Search(ctx, "operational_memory", queryVector, 3)
	if err == nil {
		for _, res := range memoryResults {
			mem, err := e.contextRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Memories = append(retrievedContext.Memories, *mem)
			}
		}
	}

	return retrievedContext, nil
}
