-- Add optional subscription plan presentation and trial configuration metadata.
-- Existing plans retain their current visibility and limited/disabled trial behavior.
ALTER TABLE website_pricing_plans
    ADD COLUMN visibility VARCHAR(10) NOT NULL DEFAULT 'public',
    ADD COLUMN is_most_popular BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN billing_type VARCHAR(20) NOT NULL DEFAULT 'per_user',
    ADD COLUMN trial_mode VARCHAR(12) NOT NULL DEFAULT 'disabled';

UPDATE website_pricing_plans
SET trial_mode = CASE WHEN trial_days > 0 THEN 'limited' ELSE 'disabled' END,
    is_most_popular = is_featured;

ALTER TABLE website_pricing_plans
    ADD CONSTRAINT website_pricing_plans_visibility_check CHECK (visibility IN ('public','hidden')),
    ADD CONSTRAINT website_pricing_plans_billing_type_check CHECK (billing_type IN ('per_user','per_workspace')),
    ADD CONSTRAINT website_pricing_plans_trial_mode_check CHECK (trial_mode IN ('disabled','limited','unlimited'));
