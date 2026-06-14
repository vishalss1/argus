-- migrations/000037_add_workspace_id_to_telemetry_tables.up.sql

ALTER TABLE events ADD COLUMN IF NOT EXISTS workspace_id UUID;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS workspace_id UUID;
ALTER TABLE operational_memory ADD COLUMN IF NOT EXISTS workspace_id UUID;

CREATE INDEX IF NOT EXISTS idx_events_workspace_id ON events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_alerts_workspace_id ON alerts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_operational_memory_workspace_id ON operational_memory(workspace_id);
