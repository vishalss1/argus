CREATE TABLE policies (
    id UUID PRIMARY KEY,
    action TEXT NOT NULL UNIQUE,
    allowed_devices UUID[] NOT NULL,
    requires_approval BOOLEAN NOT NULL DEFAULT TRUE,
    max_per_day INTEGER NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE action_execution_records (
    id UUID PRIMARY KEY,
    action TEXT NOT NULL,
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE SET NULL,
    status TEXT NOT NULL,
    suggested_by TEXT NOT NULL,
    approved_by TEXT,
    metadata TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_execution_device_id ON action_execution_records(device_id);
CREATE INDEX idx_execution_status ON action_execution_records(status);
CREATE INDEX idx_execution_created_at ON action_execution_records(created_at DESC);
