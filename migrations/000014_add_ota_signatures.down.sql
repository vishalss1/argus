DROP INDEX IF EXISTS idx_firmware_artifacts_signing_key_id;

ALTER TABLE firmware_artifacts
    DROP CONSTRAINT IF EXISTS firmware_artifacts_signature_complete_check;

ALTER TABLE firmware_artifacts
    DROP COLUMN IF EXISTS signing_key_id,
    DROP COLUMN IF EXISTS signature,
    DROP COLUMN IF EXISTS signature_alg;
