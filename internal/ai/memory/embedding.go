package memory

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
)

type EmbeddingTask struct {
	Type   string // "memory" or "event"
	Memory *ctxdomain.OperationalMemory
	Event  *event.Event
}

type EmbeddingService struct {
	provider    embedding.Provider
	vectorStore *postgres.VectorStore
	taskChan    chan EmbeddingTask
	wg          *sync.WaitGroup
}

func NewEmbeddingService(provider embedding.Provider, vectorStore *postgres.VectorStore, queueSize int) *EmbeddingService {
	return &EmbeddingService{
		provider:    provider,
		vectorStore: vectorStore,
		taskChan:    make(chan EmbeddingTask, queueSize),
	}
}

func (s *EmbeddingService) StartWorkers(ctx context.Context, workers int, wg *sync.WaitGroup) {
	s.wg = wg
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go s.worker(ctx)
	}
}

func (s *EmbeddingService) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-s.taskChan:
			ai.EmbeddingQueueDepth.Dec()
			s.processTask(ctx, task)
		}
	}
}

func (s *EmbeddingService) processTask(ctx context.Context, task EmbeddingTask) {
	err := s.withRetry(3, func() error {
		if task.Type == "memory" {
			return s.EmbedMemory(ctx, *task.Memory)
		} else if task.Type == "event" {
			return s.EmbedEvent(ctx, *task.Event)
		}
		return nil
	})

	if err != nil {
		ai.EmbeddingFailuresTotal.Inc()
		log.Printf("[EMBEDDING QUEUE] Permanent failure embedding %s (will be picked up by reconciliation): %v", task.Type, err)
	} else {
		ai.EmbeddingSuccessesTotal.Inc()
	}
}

func (s *EmbeddingService) withRetry(maxRetries int, fn func() error) error {
	var err error
	delay := 100 * time.Millisecond
	for i := 0; i < maxRetries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
}

func (s *EmbeddingService) EnqueueMemory(mem ctxdomain.OperationalMemory) {
	select {
	case s.taskChan <- EmbeddingTask{Type: "memory", Memory: &mem}:
		ai.EmbeddingQueueDepth.Inc()
	default:
		ai.EmbeddingDroppedTasksTotal.Inc()
		log.Printf("[EMBEDDING QUEUE] Queue full, dropping memory %s (will be picked up by reconciliation)", mem.ID)
	}
}

func (s *EmbeddingService) EnqueueEvent(ev event.Event) {
	select {
	case s.taskChan <- EmbeddingTask{Type: "event", Event: &ev}:
		ai.EmbeddingQueueDepth.Inc()
	default:
		ai.EmbeddingDroppedTasksTotal.Inc()
		log.Printf("[EMBEDDING QUEUE] Queue full, dropping event %s (will be picked up by reconciliation)", ev.ID)
	}
}

func (s *EmbeddingService) EmbedMemory(ctx context.Context, mem ctxdomain.OperationalMemory) error {
	text := fmt.Sprintf("%s: %s", mem.Type, mem.Summary)
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("generate memory embedding: %w", err)
	}

	return s.vectorStore.UpdateEmbedding(ctx, "operational_memory", mem.ID, vec)
}

func (s *EmbeddingService) EmbedEvent(ctx context.Context, ev event.Event) error {
	text := fmt.Sprintf("%s: %s", ev.Type, ev.Summary)
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("generate event embedding: %w", err)
	}

	return s.vectorStore.UpdateEmbedding(ctx, "events", ev.ID, vec)
}

func (s *EmbeddingService) Backfill(ctx context.Context, contextRepo *postgres.ContextRepository, eventRepo *postgres.EventRepository) error {
	var totalProcessed int64
	var totalFailed int64

	// 1. Backfill operational_memory
	memPage := 0
	for {
		select {
		case <-ctx.Done():
			ai.ReconciliationTotal.WithLabelValues("failure").Inc()
			return ctx.Err()
		default:
		}

		memIDs, err := s.vectorStore.ListIDsWithoutEmbedding(ctx, "operational_memory")
		if err != nil {
			log.Printf("[EMBEDDING BACKFILL] Failed to list memory IDs: %v", err)
			totalFailed++
			break
		}
		if len(memIDs) == 0 {
			break
		}

		log.Printf("[EMBEDDING BACKFILL] batch %d: found %d operational_memory records without embeddings", memPage+1, len(memIDs))
		for _, id := range memIDs {
			mem, err := contextRepo.GetByID(ctx, id)
			if err != nil {
				log.Printf("[EMBEDDING BACKFILL] Failed to get memory %s: %v", id, err)
				continue
			}
			s.EnqueueMemory(*mem)
			totalProcessed++
		}

		memPage++
		// Safety limit to avoid infinite looping
		if memPage >= 10 {
			log.Printf("[EMBEDDING BACKFILL] Reached max safety page limit for operational_memory (10,000 records)")
			break
		}
	}

	// 2. Backfill events
	evPage := 0
	for {
		select {
		case <-ctx.Done():
			ai.ReconciliationTotal.WithLabelValues("failure").Inc()
			return ctx.Err()
		default:
		}

		evIDs, err := s.vectorStore.ListIDsWithoutEmbedding(ctx, "events")
		if err != nil {
			log.Printf("[EMBEDDING BACKFILL] Failed to list event IDs: %v", err)
			totalFailed++
			break
		}
		if len(evIDs) == 0 {
			break
		}

		log.Printf("[EMBEDDING BACKFILL] batch %d: found %d event records without embeddings", evPage+1, len(evIDs))
		for _, id := range evIDs {
			ev, err := eventRepo.GetByID(ctx, id)
			if err != nil {
				log.Printf("[EMBEDDING BACKFILL] Failed to get event %s: %v", id, err)
				continue
			}
			s.EnqueueEvent(*ev)
			totalProcessed++
		}

		evPage++
		// Safety limit to avoid infinite looping
		if evPage >= 10 {
			log.Printf("[EMBEDDING BACKFILL] Reached max safety page limit for events (10,000 records)")
			break
		}
	}

	if totalFailed > 0 {
		ai.ReconciliationTotal.WithLabelValues("failure").Inc()
	} else if totalProcessed > 0 {
		ai.ReconciliationTotal.WithLabelValues("success").Inc()
	} else {
		ai.ReconciliationTotal.WithLabelValues("no_op").Inc()
	}

	return nil
}

