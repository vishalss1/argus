ALTER TABLE ota_deployments
    DROP CONSTRAINT IF EXISTS ota_deployments_status_check;

ALTER TABLE ota_deployments
    ADD COLUMN IF NOT EXISTS progress INTEGER NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS downloading_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS flashing_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS rebooting_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS timed_out_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS failure_reason TEXT;

ALTER TABLE ota_deployments
    ADD CONSTRAINT ota_deployments_status_check
    CHECK (status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting', 'acked', 'nacked', 'timeout'));

UPDATE ota_deployments
SET completed_at = COALESCE(completed_at, acknowledged_at),
    progress = CASE WHEN status = 'acked' THEN 100 ELSE progress END
WHERE status = 'acked';

UPDATE ota_deployments
SET failed_at = COALESCE(failed_at, acknowledged_at),
    failure_reason = COALESCE(failure_reason, result_message)
WHERE status = 'nacked';

CREATE TABLE IF NOT EXISTS ota_deployment_events (
    id BIGSERIAL PRIMARY KEY,
    deployment_id UUID NOT NULL REFERENCES ota_deployments(id) ON DELETE CASCADE,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting', 'acked', 'nacked', 'timeout')),
    progress INTEGER CHECK (progress >= 0 AND progress <= 100),
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ota_deployment_events_deployment_created_at ON ota_deployment_events(deployment_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_ota_deployment_events_device_created_at ON ota_deployment_events(device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_updated_at ON ota_deployments(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_device_active ON ota_deployments(device_id, created_at ASC)
    WHERE status IN ('pending', 'available', 'downloading', 'flashing', 'rebooting');
