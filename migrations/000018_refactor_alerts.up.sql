-- Phase 6: Refactor alerts table to decouple from telemetry table
-- 1. Remove foreign key constraint to telemetry
ALTER TABLE alerts DROP CONSTRAINT IF EXISTS alerts_telemetry_id_fkey;

-- 2. Make telemetry_id nullable
ALTER TABLE alerts ALTER COLUMN telemetry_id DROP NOT NULL;

-- 3. Add severity column
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS severity VARCHAR(20) NOT NULL DEFAULT 'warning';

-- 4. Update existing alerts to have a default severity if needed (they already have a default above)
