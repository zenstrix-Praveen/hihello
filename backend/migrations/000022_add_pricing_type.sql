-- Distinguish free and contact-sales plans from fixed-price plans without
-- changing the shared pricing catalogue or existing checkout identifiers.
ALTER TABLE website_pricing_plans
    ADD COLUMN IF NOT EXISTS pricing_type VARCHAR(20) NOT NULL DEFAULT 'fixed';

UPDATE website_pricing_plans
SET pricing_type = CASE
    WHEN billing_period = 'custom' THEN 'custom'
    WHEN price = 0 THEN 'free'
    ELSE 'fixed'
END;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'website_pricing_plans_pricing_type_check'
          AND conrelid = 'website_pricing_plans'::regclass
    ) THEN
        ALTER TABLE website_pricing_plans
            ADD CONSTRAINT website_pricing_plans_pricing_type_check
            CHECK (pricing_type IN ('free','fixed','custom'));
    END IF;
END $$;
