-- Resize embedding columns from 768 to 384 dimensions.
--
-- Mirrors the extension guard in 000010_add_pgvector.up.sql: this database is
-- expected to boot without pgvector installed, so an unguarded
-- `vector(384)` here would fail the whole migration (and with it, service
-- startup) on any instance where the extension was never created.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector') THEN
        ALTER TABLE events ALTER COLUMN embedding TYPE vector(384) USING NULL;
        ALTER TABLE operational_memory ALTER COLUMN embedding TYPE vector(384) USING NULL;
    ELSE
        RAISE NOTICE 'pgvector extension not available, skipping embedding dimension resize.';
    END IF;
END;
$$;
