CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    metric TEXT NOT NULL,
    operator TEXT NOT NULL CHECK (operator IN ('>', '>=', '<', '<=', '==', '!=')),
    threshold DOUBLE PRECISION NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rules_enabled ON rules(enabled);
CREATE INDEX IF NOT EXISTS idx_rules_metric ON rules(metric);

CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY,
    rule_id UUID NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    telemetry_id UUID NOT NULL REFERENCES telemetry(id) ON DELETE CASCADE,
    metric TEXT NOT NULL,
    operator TEXT NOT NULL,
    threshold DOUBLE PRECISION NOT NULL,
    observed_value DOUBLE PRECISION NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_device_created_at ON alerts(device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_rule_created_at ON alerts(rule_id, created_at DESC);
