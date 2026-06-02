-- Phase 6: Session Events Store
CREATE TABLE IF NOT EXISTS session_events (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL, -- DEVICE_ONLINE, DEVICE_OFFLINE, LOW_BATTERY, etc.
    severity VARCHAR(20) NOT NULL,    -- info, warning, critical
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_session_events_session_id ON session_events(session_id);
CREATE INDEX IF NOT EXISTS idx_session_events_device_id ON session_events(device_id);
CREATE INDEX IF NOT EXISTS idx_session_events_type ON session_events(event_type);
