DROP INDEX IF EXISTS idx_rules_workspace_id;

ALTER TABLE rules DROP COLUMN IF EXISTS workspace_id;
