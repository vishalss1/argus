CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE events ADD COLUMN embedding vector(768);
ALTER TABLE incidents ADD COLUMN embedding vector(768);
ALTER TABLE operational_memory ADD COLUMN embedding vector(768);

CREATE INDEX idx_events_embedding ON events USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_incidents_embedding ON incidents USING hnsw (embedding vector_cosine_ops);
CREATE INDEX idx_operational_memory_embedding ON operational_memory USING hnsw (embedding vector_cosine_ops);
