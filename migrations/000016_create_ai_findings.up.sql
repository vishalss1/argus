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
