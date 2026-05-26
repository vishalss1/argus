package vectorstore

import (
	"context"
)

type SearchResult struct {
	ID       string
	Score    float32
	Metadata map[string]interface{}
}

type Store interface {
	Search(ctx context.Context, table string, queryVector []float32, limit int) ([]SearchResult, error)
}
