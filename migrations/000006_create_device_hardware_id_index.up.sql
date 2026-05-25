CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_hardware_id
    ON devices ((metadata->>'hardware_id'))
    WHERE metadata ? 'hardware_id';
