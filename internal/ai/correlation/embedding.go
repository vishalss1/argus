package correlation

import (
	"context"
	"fmt"

	"github.com/vishalss1/argus/internal/domain/incident"
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

func (s *EmbeddingService) EmbedIncident(ctx context.Context, inc incident.Incident) error {
	text := fmt.Sprintf("%s: %s. %s", inc.Severity, inc.Title, inc.Summary)
	vec, err := s.provider.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("generate embedding: %w", err)
	}

	return s.vectorStore.UpdateEmbedding(ctx, "incidents", inc.ID, vec)
}
