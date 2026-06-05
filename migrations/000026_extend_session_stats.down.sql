-- migrations/000026_extend_session_stats.down.sql
ALTER TABLE session_statistics 
DROP COLUMN IF EXISTS average_battery,
DROP COLUMN IF EXISTS minimum_battery,
DROP COLUMN IF EXISTS maximum_battery,
DROP COLUMN IF EXISTS average_temperature,
DROP COLUMN IF EXISTS minimum_temperature,
DROP COLUMN IF EXISTS maximum_temperature,
DROP COLUMN IF EXISTS distance_travelled,
DROP COLUMN IF EXISTS device_participation_count,
DROP COLUMN IF EXISTS command_count,
DROP COLUMN IF EXISTS anomaly_count;
