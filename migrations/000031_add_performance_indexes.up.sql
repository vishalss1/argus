CREATE INDEX IF NOT EXISTS idx_events_device_id_created_at ON events (device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_operational_memory_device_id_timestamp ON operational_memory (device_id, timestamp DESC);
