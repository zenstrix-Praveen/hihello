CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ NULL;

-- Existing active accounts predate email verification and remain usable.
UPDATE users
SET email_verified_at = COALESCE(created_at, NOW())
WHERE status = 'active' AND email_verified_at IS NULL;

CREATE TABLE IF NOT EXISTS auth_otp_codes (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    subject_type VARCHAR(16) NOT NULL CHECK (subject_type IN ('customer','admin')),
    subject_id BIGINT NOT NULL,
    destination VARCHAR(255) NOT NULL,
    channel VARCHAR(16) NOT NULL DEFAULT 'email' CHECK (channel IN ('email','sms')),
    purpose VARCHAR(40) NOT NULL CHECK (purpose IN ('customer_registration','customer_password_reset','admin_password_reset')),
    code_hash TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ NULL,
    verified_at TIMESTAMPTZ NULL,
    verification_token_hash TEXT NULL,
    verification_token_used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_auth_otp_lookup ON auth_otp_codes(subject_type, destination, purpose, created_at DESC);
