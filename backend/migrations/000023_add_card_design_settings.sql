ALTER TABLE digital_cards
    ADD COLUMN IF NOT EXISTS design_settings JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE website_templates
    ADD COLUMN IF NOT EXISTS design_settings JSONB NOT NULL DEFAULT '{}'::jsonb;
