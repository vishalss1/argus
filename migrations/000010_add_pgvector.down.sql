DROP INDEX IF EXISTS idx_operational_memory_embedding;
DROP INDEX IF EXISTS idx_incidents_embedding;
DROP INDEX IF EXISTS idx_events_embedding;

ALTER TABLE operational_memory DROP COLUMN IF EXISTS embedding;
ALTER TABLE incidents DROP COLUMN IF EXISTS embedding;
ALTER TABLE events DROP COLUMN IF EXISTS embedding;

DROP EXTENSION IF EXISTS vector;
