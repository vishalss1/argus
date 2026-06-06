-- Migration to drop revoked_at column from refresh_tokens
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS revoked_at;
