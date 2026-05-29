package query

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/domain/incident"
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

Output MUST be a valid JSON object with the following keys:
- "summary": A clear, concise answer to the user's query (string).
- "confidence": A float between 0 and 1 representing your certainty.
- "evidence": A list of specific event IDs, timestamps, or metric values used to form the answer (array of strings).
- "suggested_actions": A list of recommended operational steps (array of strings).

Example Response:
{
  "summary": "The device is overheating.",
  "confidence": 0.95,
  "evidence": ["Event 123: Temp reached 95C"],
  "suggested_actions": ["Reboot device", "Check cooling fan"]
}

If no relevant information is found in the context, set "summary" to "No matching information found in the current operational context." and "confidence" to 0.`

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
	retrievedContext := &Context{}
	seenIDs := make(map[string]bool)

	// 1. Keyword/ID Extraction (Direct Lookup)
	// Match UUID-like patterns: 8-4-4-4-12
	uuidRegex := regexp.MustCompile(`[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}`)
	matches := uuidRegex.FindAllString(queryString, -1)

	for _, id := range matches {
		log.Printf("[QUERY ENGINE] direct ID lookup for: %s", id)
		// Try event lookup
		if ev, err := e.eventRepo.GetByID(ctx, id); err == nil {
			retrievedContext.Events = append(retrievedContext.Events, *ev)
			seenIDs[ev.ID] = true
		}
		// Try incident lookup
		if inc, err := e.incidentRepo.GetByID(ctx, id); err == nil {
			retrievedContext.Incidents = append(retrievedContext.Incidents, *inc)
			seenIDs[inc.ID] = true
		}
	}

	// 2. Vector Search (Semantic Retrieval)
	queryVector, err := e.embeddingProvider.Embed(ctx, queryString)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	log.Printf("[QUERY ENGINE] performing vector search for query: %q", queryString)

	// 2.1 Search events
	eventResults, err := e.vectorStore.Search(ctx, "events", queryVector, 5)
	if err == nil {
		log.Printf("[QUERY ENGINE] found %d semantic event matches", len(eventResults))
		for _, res := range eventResults {
			if seenIDs[res.ID] {
				continue
			}
			ev, err := e.eventRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Events = append(retrievedContext.Events, *ev)
				seenIDs[ev.ID] = true
			}
		}
	} else {
		log.Printf("[QUERY ENGINE] event vector search failed: %v", err)
	}

	// 2.2 Search incidents
	incidentResults, err := e.vectorStore.Search(ctx, "incidents", queryVector, 3)
	if err == nil {
		log.Printf("[QUERY ENGINE] found %d semantic incident matches", len(incidentResults))
		for _, res := range incidentResults {
			if seenIDs[res.ID] {
				continue
			}
			inc, err := e.incidentRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Incidents = append(retrievedContext.Incidents, *inc)
				seenIDs[inc.ID] = true
			}
		}
	}

	// 2.3 Search operational memory
	memoryResults, err := e.vectorStore.Search(ctx, "operational_memory", queryVector, 3)
	if err == nil {
		log.Printf("[QUERY ENGINE] found %d semantic memory matches", len(memoryResults))
		for _, res := range memoryResults {
			if seenIDs[res.ID] {
				continue
			}
			mem, err := e.contextRepo.GetByID(ctx, res.ID)
			if err == nil {
				retrievedContext.Memories = append(retrievedContext.Memories, *mem)
				seenIDs[mem.ID] = true
			}
		}
	}

	if len(seenIDs) == 0 {
		log.Printf("[QUERY ENGINE] NO MATCHES FOUND. Query Embedding (truncated): %v...", queryVector[:5])
	}

	return retrievedContext, nil
}
