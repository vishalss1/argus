-- migrations/000027_create_session_artifacts.up.sql
CREATE TABLE IF NOT EXISTS session_artifacts (
    session_id UUID PRIMARY KEY REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    report_version VARCHAR(50) NOT NULL DEFAULT '2.0',
    artifact_json JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_session_artifacts_workspace_id ON session_artifacts(workspace_id);
