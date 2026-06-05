-- Drop redundant historical tables made obsolete by compiled Session Artifact
DROP TABLE IF EXISTS session_reports CASCADE;
DROP TABLE IF EXISTS session_ai_reports CASCADE;
DROP TABLE IF EXISTS session_events CASCADE;
DROP TABLE IF EXISTS session_alerts CASCADE;
