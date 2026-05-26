package semantic

import (
	"context"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/event"
	"github.com/vishalss1/argus/internal/infrastructure/embedding"
	"github.com/vishalss1/argus/internal/infrastructure/postgres"
)

type EmbeddingService struct {
	provider    embedding.Provider
	vectorStore *postgres.VectorStore
}

func NewEmbeddingService(provider embedding.Provider, vectorStore *postgres.VectorStore) *EmbeddingService {
	return &EmbeddingService{
		provider:    provider,
		vectorStore: vectorStore,
	}
}

func (s *EmbeddingService) EmbedEvent(ctx context.Context, ev event.Event) error {
	text := fmt.Sprintf("%s: %s. %s", ev.Type, ev.Title, ev.Summary)
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	return s.vectorStore.UpdateEmbedding(ctx, "events", ev.ID, vec)
}
