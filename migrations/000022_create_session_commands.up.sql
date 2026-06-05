-- Phase 8: Session Commands Audit Store
CREATE TABLE IF NOT EXISTS session_commands (
    id UUID PRIMARY KEY,
    session_id UUID NOT NULL REFERENCES workspace_sessions(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    command VARCHAR(50) NOT NULL,
    issued_by UUID, -- Placeholder for user tracking
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', -- PENDING, SENT, ACKED, NACKED, TIMEOUT
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_session_commands_session_id ON session_commands(session_id);
CREATE INDEX IF NOT EXISTS idx_session_commands_device_id ON session_commands(device_id);
CREATE INDEX IF NOT EXISTS idx_session_commands_status ON session_commands(status);
