-- Phase 1: Re-enable raw telemetry table writes
DROP TRIGGER IF EXISTS telemetry_read_only ON telemetry;
DROP FUNCTION IF EXISTS block_telemetry_writes();
