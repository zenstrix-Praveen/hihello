-- Preserve existing monetary values while allowing three-decimal currencies
-- (BHD, JOD, KWD, OMR) to be stored without truncation.
ALTER TABLE website_pricing_plans
    ALTER COLUMN price TYPE NUMERIC(12,3);

ALTER TABLE subscriptions
    ALTER COLUMN amount TYPE NUMERIC(12,3);

ALTER TABLE payments
    ALTER COLUMN amount TYPE NUMERIC(12,3);

ALTER TABLE payment_transactions
    ALTER COLUMN amount TYPE NUMERIC(12,3);
