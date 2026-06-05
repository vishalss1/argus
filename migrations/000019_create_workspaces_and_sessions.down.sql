DROP TABLE IF EXISTS workspace_sessions;
ALTER TABLE devices DROP COLUMN IF EXISTS workspace_id;
DROP TABLE IF EXISTS workspaces;
