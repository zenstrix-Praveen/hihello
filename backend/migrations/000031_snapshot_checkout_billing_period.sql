-- Preserve the purchased period even if an administrator later edits the plan.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS billing_period VARCHAR(20)
    CHECK (billing_period IN ('monthly','yearly','lifetime'));

-- Existing activated payments have an authoritative subscription snapshot.
UPDATE payments pay SET billing_period = s.billing_period
FROM subscriptions s
WHERE pay.subscription_id = s.id AND pay.billing_period IS NULL
  AND s.billing_period IN ('monthly','yearly','lifetime');

-- Legacy open orders did not save their period. Retain the known current value.
UPDATE payments pay SET billing_period = p.billing_period
FROM website_pricing_plans p
WHERE pay.plan_id = p.id AND pay.billing_period IS NULL
  AND p.billing_period IN ('monthly','yearly','lifetime');
