package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/pgvector/pgvector-go"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/vectorstore"
)

type VectorStore struct {
	db *sql.DB
}

func NewVectorStore(db *sql.DB) *VectorStore {
	return &VectorStore{db: db}
}

func (s *VectorStore) Search(ctx context.Context, table string, queryVector []float32, limit int) ([]vectorstore.SearchResult, error) {
	// Sanitize table name to prevent SQL injection (simple check for allowed tables)
	allowedTables := map[string]bool{
		"events":             true,
		"incidents":          true,
		"operational_memory": true,
	}
	if !allowedTables[table] {
		return nil, fmt.Errorf("table %s not allowed for vector search", table)
	}

	ai.VectorQueriesTotal.WithLabelValues(table).Inc()
	log.Printf("[VECTOR STORE] searching %s with limit %d", table, limit)

	query := fmt.Sprintf(`
		SELECT id, embedding <=> $1 as distance
		FROM %s
		WHERE embedding IS NOT NULL
		ORDER BY distance
		LIMIT $2
	`, table)

	rows, err := s.db.QueryContext(ctx, query, pgvector.NewVector(queryVector), limit)
	if err != nil {
		log.Printf("[VECTOR STORE] search failed on %s: %v", table, err)
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer rows.Close()

	var results []vectorstore.SearchResult
	for rows.Next() {
		var res vectorstore.SearchResult
		var distance float32
		if err := rows.Scan(&res.ID, &distance); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		res.Score = 1.0 - distance // Cosine similarity approx
		results = append(results, res)
	}

	log.Printf("[VECTOR STORE] found %d matches in %s", len(results), table)
	return results, nil
}

func (s *VectorStore) UpdateEmbedding(ctx context.Context, table string, id string, embedding []float32) error {
	allowedTables := map[string]bool{
		"events":             true,
		"incidents":          true,
		"operational_memory": true,
	}
	if !allowedTables[table] {
		return fmt.Errorf("table %s not allowed for embedding update", table)
	}

	query := fmt.Sprintf("UPDATE %s SET embedding = $1 WHERE id = $2", table)
	_, err := s.db.ExecContext(ctx, query, pgvector.NewVector(embedding), id)
	if err == nil {
		log.Printf("[VECTOR STORE] updated embedding for %s:%s", table, id)
	} else {
		log.Printf("[VECTOR STORE] failed to update embedding for %s:%s: %v", table, id, err)
	}
	return err
}
