package embedding

import "context"

// Provider defines the interface for generating vector embeddings.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
