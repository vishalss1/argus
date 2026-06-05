-- Down migration for Phase 6 refactor
ALTER TABLE alerts ADD CONSTRAINT alerts_telemetry_id_fkey FOREIGN KEY (telemetry_id) REFERENCES telemetry(id) ON DELETE CASCADE;
ALTER TABLE alerts ALTER COLUMN telemetry_id SET NOT NULL;
ALTER TABLE alerts DROP COLUMN IF EXISTS severity;
