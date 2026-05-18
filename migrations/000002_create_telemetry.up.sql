CREATE TABLE IF NOT EXISTS telemetry (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    recorded_at TIMESTAMPTZ NOT NULL,
    metrics JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_device_recorded_at ON telemetry(device_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_recorded_at ON telemetry(recorded_at DESC);
