CREATE TABLE operational_memory (
    id UUID PRIMARY KEY,
    device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    summary TEXT NOT NULL,
    data JSONB NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_operational_memory_device_id ON operational_memory(device_id);
CREATE INDEX idx_operational_memory_type ON operational_memory(type);
CREATE INDEX idx_operational_memory_timestamp ON operational_memory(timestamp DESC);
