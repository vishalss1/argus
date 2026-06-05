-- Phase 1: Session-Centric Domain Model
-- 1. Create Workspaces
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Link Devices to Workspaces
ALTER TABLE devices ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_devices_workspace_id ON devices(workspace_id);

-- 3. Create Sessions
CREATE TABLE IF NOT EXISTS workspace_sessions (
    id UUID PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED', -- CREATED, RUNNING, COMPLETED, FAILED, CANCELLED
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    created_by UUID, -- Placeholder for future auth integration
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workspace_sessions_workspace_id ON workspace_sessions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_workspace_sessions_status ON workspace_sessions(status);
