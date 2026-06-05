-- Phase 1: Deprecate raw telemetry table
-- This migration marks the telemetry table as read-only to prevent further writes
-- during the transition to the streaming pipeline.

CREATE OR REPLACE FUNCTION block_telemetry_writes() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Table telemetry is read-only (Refactor Phase 1). Use the streaming pipeline (Redpanda/Kafka) instead.';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS telemetry_read_only ON telemetry;

CREATE TRIGGER telemetry_read_only
BEFORE INSERT OR UPDATE OR DELETE ON telemetry
FOR EACH ROW EXECUTE FUNCTION block_telemetry_writes();
