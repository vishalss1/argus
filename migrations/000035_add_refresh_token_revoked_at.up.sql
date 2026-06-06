-- Migration to add revoked_at column to refresh_tokens to support token rotation grace periods
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;
