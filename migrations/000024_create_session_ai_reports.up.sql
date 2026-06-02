-- Phase 11: Session AI Summary Store
CREATE TABLE IF NOT EXISTS session_ai_reports (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    summary_text TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_session_ai_reports_session_id ON session_ai_reports(session_id);
