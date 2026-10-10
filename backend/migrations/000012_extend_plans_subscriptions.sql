-- Extend the existing Website Pricing catalogue and subscription records for
-- administrative plan management. These are the existing source-of-truth tables.
ALTER TABLE website_pricing_plans
    ADD COLUMN public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN code VARCHAR(100),
    ADD COLUMN trial_days INT NOT NULL DEFAULT 0,
    ADD COLUMN limits JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE website_pricing_plans
SET code = 'plan-' || id::text
WHERE code IS NULL OR btrim(code) = '';

ALTER TABLE website_pricing_plans ALTER COLUMN code SET NOT NULL;
CREATE UNIQUE INDEX idx_website_pricing_plans_public_id ON website_pricing_plans(public_id);
CREATE UNIQUE INDEX idx_website_pricing_plans_code ON website_pricing_plans(code);
CREATE INDEX idx_website_pricing_plans_enabled_order ON website_pricing_plans(is_enabled, sort_order, id);
ALTER TABLE website_pricing_plans
    ADD CONSTRAINT website_pricing_plans_billing_period_check
    CHECK (billing_period IN ('monthly','yearly','lifetime','custom')),
    ADD CONSTRAINT website_pricing_plans_trial_days_check CHECK (trial_days >= 0);

ALTER TABLE subscriptions
    ADD COLUMN public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN ends_at TIMESTAMPTZ NULL,
    ADD COLUMN trial_ends_at TIMESTAMPTZ NULL,
    ADD COLUMN amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    ADD COLUMN currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    ADD COLUMN billing_period VARCHAR(30) NOT NULL DEFAULT 'monthly',
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE subscriptions s
SET amount = COALESCE(p.price, 0),
    currency = COALESCE(p.currency, 'USD'),
    billing_period = COALESCE(p.billing_period, 'monthly'),
    ends_at = s.renews_at,
    status = CASE s.status WHEN 'trialing' THEN 'trial' WHEN 'inactive' THEN 'expired' ELSE s.status END
FROM website_pricing_plans p
WHERE s.plan_id = p.id;

UPDATE subscriptions SET status = CASE status WHEN 'trialing' THEN 'trial' WHEN 'inactive' THEN 'expired' ELSE status END;
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS subscriptions_status_check;
ALTER TABLE subscriptions
    ADD CONSTRAINT subscriptions_status_check CHECK (status IN ('trial','active','past_due','cancelled','expired')),
    ADD CONSTRAINT subscriptions_billing_period_check CHECK (billing_period IN ('monthly','yearly','lifetime','custom')),
    ADD CONSTRAINT subscriptions_amount_check CHECK (amount >= 0);

CREATE UNIQUE INDEX idx_subscriptions_public_id ON subscriptions(public_id);
CREATE INDEX idx_subscriptions_plan_id ON subscriptions(plan_id);
CREATE INDEX idx_subscriptions_status ON subscriptions(status);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_organization_id ON subscriptions(organization_id);
CREATE INDEX idx_subscriptions_created_at ON subscriptions(created_at DESC);
