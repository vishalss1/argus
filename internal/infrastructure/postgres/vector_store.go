package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/pgvector/pgvector-go"
	"github.com/vishalss1/argus/internal/infrastructure/ai"
	"github.com/vishalss1/argus/internal/infrastructure/vectorstore"
)

var ErrVectorSearchDisabled = errors.New("vector search and embeddings are disabled because pgvector is not installed")

type VectorStore struct {
	db      *sql.DB
	enabled bool
}

func NewVectorStore(db *sql.DB) *VectorStore {
	store := &VectorStore{db: db}
	store.verifyExtension()
	return store
}

func (s *VectorStore) verifyExtension() {
	var dbName string
	s.db.QueryRow("SELECT current_database()").Scan(&dbName)

	var version string
	err := s.db.QueryRow("SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&version)
	if err != nil {
		s.enabled = false
		log.Printf("[VECTOR STORE] INFO: pgvector extension is NOT installed in %s. Semantic search and embeddings are disabled.", dbName)
		return
	}

	// Verify columns exist
	tables := []string{"events", "incidents", "operational_memory"}
	for _, table := range tables {
		var exists bool
		query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='%s' AND column_name='embedding')", table)
		if err := s.db.QueryRow(query).Scan(&exists); err != nil || !exists {
			s.enabled = false
			log.Printf("[VECTOR STORE] WARNING: table %s is missing 'embedding' column. Semantic search disabled.", table)
			return
		}
	}

	s.enabled = true
	log.Printf("[VECTOR STORE] pgvector enabled in %s (version %s). 768 dimensions (nomic-embed-text) confirmed.", dbName, version)
}

func (s *VectorStore) Search(ctx context.Context, table string, queryVector []float32, limit int) ([]vectorstore.SearchResult, error) {
	if !s.enabled {
		return nil, ErrVectorSearchDisabled
	}

	// Sanitize table name
	allowedTables := map[string]bool{
		"events":             true,
		"incidents":          true,
		"operational_memory": true,
	}
	if !allowedTables[table] {
		return nil, fmt.Errorf("table %s not allowed for vector search", table)
	}

	ai.VectorQueriesTotal.WithLabelValues(table).Inc()

	query := fmt.Sprintf(`
		SELECT id, embedding <=> $1 as distance
		FROM %s
		WHERE embedding IS NOT NULL
		ORDER BY distance
		LIMIT $2
	`, table)

	rows, err := s.db.QueryContext(ctx, query, pgvector.NewVector(queryVector), limit)
	if err != nil {
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
		res.Score = 1.0 - distance
		results = append(results, res)
	}

	return results, nil
}

func (s *VectorStore) UpdateEmbedding(ctx context.Context, table string, id string, embedding []float32) error {
	if !s.enabled {
		return ErrVectorSearchDisabled
	}

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
	return err
}
