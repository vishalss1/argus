-- Phase 9: Session Statistics
CREATE TABLE IF NOT EXISTS session_statistics (
    session_id UUID PRIMARY KEY REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    duration_seconds INT NOT NULL DEFAULT 0,
    messages_processed INT NOT NULL DEFAULT 0,
    alerts_count INT NOT NULL DEFAULT 0,
    critical_events INT NOT NULL DEFAULT 0,
    uptime_percentage FLOAT NOT NULL DEFAULT 100.0,
    average_latency_ms FLOAT NOT NULL DEFAULT 0.0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Phase 10: Session Reports
CREATE TABLE IF NOT EXISTS session_reports (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    report_json JSONB NOT NULL DEFAULT '{}',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_session_reports_session_id ON session_reports(session_id);
