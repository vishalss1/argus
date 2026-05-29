DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        ALTER TABLE events ADD COLUMN IF NOT EXISTS embedding vector(768);
        ALTER TABLE incidents ADD COLUMN IF NOT EXISTS embedding vector(768);
        ALTER TABLE operational_memory ADD COLUMN IF NOT EXISTS embedding vector(768);

        CREATE INDEX IF NOT EXISTS idx_events_embedding ON events USING hnsw (embedding vector_cosine_ops);
        CREATE INDEX IF NOT EXISTS idx_incidents_embedding ON incidents USING hnsw (embedding vector_cosine_ops);
        CREATE INDEX IF NOT EXISTS idx_operational_memory_embedding ON operational_memory USING hnsw (embedding vector_cosine_ops);
    END IF;
END;
$$;
