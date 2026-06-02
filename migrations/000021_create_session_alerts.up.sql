-- Phase 7: Session Alerts Store
CREATE TABLE IF NOT EXISTS session_alerts (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    severity VARCHAR(20) NOT NULL, -- info, warning, critical
    message TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_session_alerts_session_id ON session_alerts(session_id);
CREATE INDEX IF NOT EXISTS idx_session_alerts_device_id ON session_alerts(device_id);
CREATE INDEX IF NOT EXISTS idx_session_alerts_status ON session_alerts(resolved);
