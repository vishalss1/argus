-- Remove raw telemetry table and its write-block trigger
DROP TABLE IF EXISTS telemetry CASCADE;
DROP FUNCTION IF EXISTS block_telemetry_writes CASCADE;

-- Remove AI findings table
DROP TABLE IF EXISTS ai_findings CASCADE;

-- Remove database-backed incidents table
DROP TABLE IF EXISTS incidents CASCADE;
