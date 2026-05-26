package memory

import (
	"context"
	"fmt"

	ctxdomain "github.com/vishalss1/argus/internal/domain/context"
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

func (s *EmbeddingService) EmbedMemory(ctx context.Context, mem ctxdomain.OperationalMemory) error {
	text := fmt.Sprintf("%s: %s", mem.Type, mem.Summary)
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("generate memory embedding: %w", err)
	}

	return s.vectorStore.UpdateEmbedding(ctx, "operational_memory", mem.ID, vec)
}
