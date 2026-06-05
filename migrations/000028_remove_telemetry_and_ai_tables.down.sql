CREATE TABLE IF NOT EXISTS telemetry (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    recorded_at TIMESTAMPTZ NOT NULL,
    metrics JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_device_recorded_at ON telemetry(device_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_telemetry_recorded_at ON telemetry(recorded_at DESC);

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    severity TEXT NOT NULL,
    status TEXT NOT NULL,
    device_ids UUID[] NOT NULL,
    event_ids UUID[] NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_created_at ON incidents(created_at DESC);

CREATE TABLE IF NOT EXISTS ai_findings (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    risk_score FLOAT NOT NULL,
    health_score INT NOT NULL,
    severity VARCHAR(20) NOT NULL,
    summary TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_findings_device_id ON ai_findings(device_id);
CREATE INDEX IF NOT EXISTS idx_ai_findings_created_at ON ai_findings(created_at DESC);
