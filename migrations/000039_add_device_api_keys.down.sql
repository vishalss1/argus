-- migrations/000039_add_device_api_keys.down.sql

DROP INDEX IF EXISTS idx_devices_api_key_prefix;

ALTER TABLE devices
DROP COLUMN IF EXISTS api_key_prefix,
DROP COLUMN IF EXISTS api_key_hash;
