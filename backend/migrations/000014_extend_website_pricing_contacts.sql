ALTER TABLE website_pricing_plans
    ADD COLUMN IF NOT EXISTS cta_text VARCHAR(150) NOT NULL DEFAULT 'Get Started',
    ADD COLUMN IF NOT EXISTS cta_url TEXT NOT NULL DEFAULT '';

ALTER TABLE contacts
    ADD COLUMN IF NOT EXISTS subject VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS message TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_contacts_website_enquiries
    ON contacts(status, created_at DESC)
    WHERE source = 'website';
