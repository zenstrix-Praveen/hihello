CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    organization_id BIGINT NULL REFERENCES organizations(id) ON DELETE SET NULL,
    subscription_id BIGINT NULL REFERENCES subscriptions(id) ON DELETE SET NULL,
    plan_id BIGINT NOT NULL REFERENCES website_pricing_plans(id) ON DELETE RESTRICT,
    provider VARCHAR(30) NOT NULL DEFAULT 'razorpay' CHECK (provider = 'razorpay'),
    provider_order_id VARCHAR(100) NOT NULL UNIQUE,
    provider_payment_id VARCHAR(100) NULL UNIQUE,
    amount NUMERIC(12,2) NOT NULL CHECK (amount >= 0),
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'created'
      CHECK (status IN ('created','pending','authorized','paid','failed','cancelled','refunded')),
    payment_method VARCHAR(30) NULL,
    bank VARCHAR(100) NULL,
    wallet VARCHAR(100) NULL,
    error_code VARCHAR(100) NULL,
    error_description TEXT NULL,
    paid_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payments_user_created ON payments(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_plan_status ON payments(plan_id, status);
CREATE INDEX IF NOT EXISTS idx_payments_status_created ON payments(status, created_at DESC);

CREATE TABLE IF NOT EXISTS payment_transactions (
    id BIGSERIAL PRIMARY KEY,
    payment_id BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    provider_event_id VARCHAR(150) NULL,
    event_type VARCHAR(100) NOT NULL,
    amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_transactions_provider_event
    ON payment_transactions(provider_event_id) WHERE provider_event_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payment_transactions_payment_created
    ON payment_transactions(payment_id, created_at DESC);

CREATE TABLE IF NOT EXISTS processed_payment_webhook_events (
    provider_event_id VARCHAR(150) PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    payload_sha256 CHAR(64) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
