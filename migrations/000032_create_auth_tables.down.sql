DROP INDEX IF EXISTS idx_auth_audit_logs_timestamp;
DROP INDEX IF EXISTS idx_workspace_members_user_id;
DROP INDEX IF EXISTS idx_workspace_members_workspace_id;
DROP INDEX IF EXISTS idx_refresh_tokens_expires_at;
DROP INDEX IF EXISTS idx_refresh_tokens_user_id;

DROP TABLE IF EXISTS auth_audit_logs;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
