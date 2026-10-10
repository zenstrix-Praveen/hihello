-- Additive migration; preserve all plan defaults and payment/subscription snapshots.
ALTER TABLE website_pricing_plans ADD COLUMN IF NOT EXISTS billing_config JSONB NOT NULL DEFAULT '{"prices":[],"providers":["razorpay"]}'::jsonb;
-- ISO fund units can have four decimals.
ALTER TABLE website_pricing_plans ALTER COLUMN price TYPE NUMERIC(18,4);
ALTER TABLE payments ALTER COLUMN amount TYPE NUMERIC(18,4);
ALTER TABLE subscriptions ALTER COLUMN amount TYPE NUMERIC(18,4);
ALTER TABLE payment_transactions ALTER COLUMN amount TYPE NUMERIC(18,4);
ALTER TABLE payments ADD COLUMN IF NOT EXISTS checkout_country VARCHAR(2) NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS checkout_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_open_checkout ON payments(user_id,provider,created_at) WHERE status IN ('created','pending');
