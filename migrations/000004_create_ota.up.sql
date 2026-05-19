CREATE TABLE IF NOT EXISTS firmware_artifacts (
    id UUID PRIMARY KEY,
    version TEXT NOT NULL,
    filename TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
    checksum_sha256 TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_firmware_artifacts_created_at ON firmware_artifacts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_firmware_artifacts_version ON firmware_artifacts(version);

CREATE TABLE IF NOT EXISTS ota_deployments (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    artifact_id UUID NOT NULL REFERENCES firmware_artifacts(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'acked', 'nacked')),
    result_message TEXT,
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ota_deployments_device_created_at ON ota_deployments(device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_status ON ota_deployments(status);
