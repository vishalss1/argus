-- migrations/000039_add_device_api_keys.up.sql

ALTER TABLE devices
ADD COLUMN IF NOT EXISTS api_key_hash bytea,
ADD COLUMN IF NOT EXISTS api_key_prefix varchar(8);

CREATE INDEX IF NOT EXISTS idx_devices_api_key_prefix ON devices(api_key_prefix);
