ALTER TABLE qr_codes
    ADD COLUMN IF NOT EXISTS public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS organization_id BIGINT NULL REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS destination_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS label VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS scan_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_scanned_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE qr_codes q
SET user_id = COALESCE(q.user_id, c.user_id),
    organization_id = COALESCE(q.organization_id, c.organization_id)
FROM digital_cards c
WHERE q.card_id = c.id;

UPDATE qr_codes SET status = 'disabled' WHERE status = 'inactive';

ALTER TABLE qr_codes DROP CONSTRAINT IF EXISTS qr_codes_status_check;
ALTER TABLE qr_codes ADD CONSTRAINT qr_codes_status_check
    CHECK (status IN ('active', 'disabled', 'archived'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_qr_codes_public_id ON qr_codes(public_id);
CREATE INDEX IF NOT EXISTS idx_qr_codes_user_id ON qr_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_qr_codes_organization_id ON qr_codes(organization_id);
CREATE INDEX IF NOT EXISTS idx_qr_codes_status ON qr_codes(status);
CREATE INDEX IF NOT EXISTS idx_qr_codes_created_at ON qr_codes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_qr_codes_last_scanned_at ON qr_codes(last_scanned_at DESC);
