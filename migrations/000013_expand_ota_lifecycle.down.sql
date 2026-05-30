DROP TABLE IF EXISTS ota_deployment_events;

ALTER TABLE ota_deployments
    DROP CONSTRAINT IF EXISTS ota_deployments_status_check;

UPDATE ota_deployments SET status = 'pending' WHERE status IN ('available', 'downloading', 'flashing', 'rebooting', 'timeout');

ALTER TABLE ota_deployments
    ADD CONSTRAINT ota_deployments_status_check
    CHECK (status IN ('pending', 'acked', 'nacked'));

ALTER TABLE ota_deployments
    DROP COLUMN IF EXISTS progress,
    DROP COLUMN IF EXISTS available_at,
    DROP COLUMN IF EXISTS downloading_at,
    DROP COLUMN IF EXISTS flashing_at,
    DROP COLUMN IF EXISTS rebooting_at,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS failed_at,
    DROP COLUMN IF EXISTS timed_out_at,
    DROP COLUMN IF EXISTS failure_reason;

DROP INDEX IF EXISTS idx_ota_deployments_updated_at;
