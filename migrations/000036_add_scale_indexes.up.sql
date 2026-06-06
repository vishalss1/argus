CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_devices_workspace_lower_name ON devices (workspace_id, lower(name));
CREATE INDEX IF NOT EXISTS idx_devices_workspace_name ON devices (workspace_id, name);
CREATE INDEX IF NOT EXISTS idx_devices_lower_name_trgm ON devices USING GIN (lower(name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_devices_hardware_id_trgm ON devices USING GIN (lower(metadata->>'hardware_id') gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_devices_hardware_id ON devices ((metadata->>'hardware_id'));
CREATE INDEX IF NOT EXISTS idx_devices_workspace_updated_at ON devices (workspace_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_devices_status_last_seen ON devices (status, last_seen);

CREATE INDEX IF NOT EXISTS idx_events_type_created_at ON events (type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_device_type_created_at ON events (device_id, type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_operational_memory_type_timestamp ON operational_memory (type, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_operational_memory_device_type_timestamp ON operational_memory (device_id, type, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_workspace_sessions_workspace_created_at ON workspace_sessions (workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workspace_sessions_workspace_status_created_at ON workspace_sessions (workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workspace_sessions_status_started_at ON workspace_sessions (status, started_at);
CREATE INDEX IF NOT EXISTS idx_session_commands_session_issued_at ON session_commands (session_id, issued_at);

CREATE INDEX IF NOT EXISTS idx_alerts_severity_created_at ON alerts (severity, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_device_created_at ON alerts (device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_commands_device_created_at ON commands (device_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_firmware_artifacts_created_at ON firmware_artifacts (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_created_at ON ota_deployments (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_device_created_at ON ota_deployments (device_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ota_deployments_device_status_created_at ON ota_deployments (device_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_ota_deployment_events_deployment_created_at ON ota_deployment_events (deployment_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_rules_enabled_created_at ON rules (enabled, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_action_execution_records_created_at ON action_execution_records (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workspaces_created_at ON workspaces (created_at DESC);
