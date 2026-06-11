-- migrations/000037_add_workspace_id_to_telemetry_tables.down.sql

DROP INDEX IF EXISTS idx_operational_memory_workspace_id;
DROP INDEX IF EXISTS idx_alerts_workspace_id;
DROP INDEX IF EXISTS idx_events_workspace_id;

ALTER TABLE operational_memory DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE alerts DROP COLUMN IF EXISTS workspace_id;
ALTER TABLE events DROP COLUMN IF EXISTS workspace_id;
