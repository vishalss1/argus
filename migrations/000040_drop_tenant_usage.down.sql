-- migrations/000040_drop_tenant_usage.down.sql
CREATE TABLE IF NOT EXISTS tenant_usage (
    tenant_id UUID NOT NULL,
    billing_month VARCHAR(7) NOT NULL,
    devices_used INT NOT NULL DEFAULT 0,
    workspaces_used INT NOT NULL DEFAULT 0,
    sessions_run INT NOT NULL DEFAULT 0,
    messages_processed BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, billing_month)
);
