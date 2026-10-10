ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS is_trial BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE subscriptions s SET is_trial=TRUE FROM website_pricing_plans p
WHERE p.id=s.plan_id AND p.price>0 AND p.pricing_type<>'free'
  AND (s.status='trial' OR s.trial_ends_at IS NOT NULL
       OR (s.amount=0 AND p.trial_mode='unlimited'));
