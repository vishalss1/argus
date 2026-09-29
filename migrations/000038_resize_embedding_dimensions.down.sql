-- Revert embedding columns to the 768 dimensions created by 000010/000012.
-- Guarded for the same reason as the up migration: this instance may not have
-- pgvector installed, in which case the columns are still at their original
-- dimension and there is nothing to revert.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        ALTER TABLE events ALTER COLUMN embedding TYPE vector(768) USING NULL;
        ALTER TABLE operational_memory ALTER COLUMN embedding TYPE vector(768) USING NULL;
    ELSE
        RAISE NOTICE 'pgvector extension not available, skipping embedding dimension revert.';
    END IF;
END;
$$;
