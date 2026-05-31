ALTER TABLE firmware_artifacts
    ADD COLUMN IF NOT EXISTS signature_alg TEXT,
    ADD COLUMN IF NOT EXISTS signature TEXT,
    ADD COLUMN IF NOT EXISTS signing_key_id TEXT;

ALTER TABLE firmware_artifacts
    ADD CONSTRAINT firmware_artifacts_signature_complete_check
    CHECK (
        (signature_alg IS NULL AND signature IS NULL AND signing_key_id IS NULL)
        OR
        (signature_alg = 'ed25519' AND signature IS NOT NULL AND signing_key_id IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS idx_firmware_artifacts_signing_key_id ON firmware_artifacts(signing_key_id);
