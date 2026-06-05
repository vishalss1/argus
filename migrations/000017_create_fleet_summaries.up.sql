CREATE TABLE IF NOT EXISTS fleet_summaries (
    id UUID PRIMARY KEY,
    active_devices INT NOT NULL,
    offline_devices INT NOT NULL,
    avg_health_score FLOAT NOT NULL,
    avg_risk_score FLOAT NOT NULL,
    high_risk_devices INT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fleet_summaries_created_at ON fleet_summaries(created_at DESC);
