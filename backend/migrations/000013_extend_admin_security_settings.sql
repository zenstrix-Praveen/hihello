ALTER TABLE admin_users
    ADD COLUMN IF NOT EXISTS failed_login_attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TABLE IF NOT EXISTS admin_security_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    password_min_length INT NOT NULL DEFAULT 12 CHECK (password_min_length BETWEEN 8 AND 128),
    require_uppercase BOOLEAN NOT NULL DEFAULT TRUE,
    require_lowercase BOOLEAN NOT NULL DEFAULT TRUE,
    require_number BOOLEAN NOT NULL DEFAULT TRUE,
    require_special BOOLEAN NOT NULL DEFAULT FALSE,
    password_expiry_days INT NULL CHECK (password_expiry_days IS NULL OR password_expiry_days BETWEEN 1 AND 3650),
    session_timeout_minutes INT NOT NULL DEFAULT 480 CHECK (session_timeout_minutes BETWEEN 5 AND 43200),
    max_failed_login_attempts INT NOT NULL DEFAULT 5 CHECK (max_failed_login_attempts BETWEEN 1 AND 100),
    lockout_duration_minutes INT NOT NULL DEFAULT 15 CHECK (lockout_duration_minutes BETWEEN 1 AND 10080),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO admin_security_settings(id) VALUES(1) ON CONFLICT(id) DO NOTHING;

ALTER TABLE admin_security_activity
    ADD COLUMN IF NOT EXISTS action VARCHAR(100),
    ADD COLUMN IF NOT EXISTS entity_type VARCHAR(100),
    ADD COLUMN IF NOT EXISTS entity_id VARCHAR(150),
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS ip_address INET,
    ADD COLUMN IF NOT EXISTS user_agent TEXT;
UPDATE admin_security_activity SET action=event_type WHERE action IS NULL;
ALTER TABLE admin_security_activity ALTER COLUMN action SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_admin_security_activity_action_created ON admin_security_activity(action, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_users_locked_until ON admin_users(locked_until) WHERE locked_until IS NOT NULL;
