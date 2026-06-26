CREATE TABLE fleets (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    node_role       VARCHAR(255) NOT NULL,
    hardware_type   VARCHAR(100) NOT NULL,
    node_prefix     VARCHAR(100) NOT NULL DEFAULT 'Node',
    firmware_version VARCHAR(100) NOT NULL DEFAULT '0.0.0',
    firmware_template TEXT        NOT NULL DEFAULT '',
    node_count      INTEGER      NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE devices
    ADD COLUMN fleet_id UUID REFERENCES fleets(id) ON DELETE CASCADE;

CREATE INDEX idx_fleets_workspace_id ON fleets(workspace_id);
CREATE INDEX idx_devices_fleet_id    ON devices(fleet_id);
