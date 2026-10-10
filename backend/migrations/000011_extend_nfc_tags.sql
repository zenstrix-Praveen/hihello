ALTER TABLE nfc_tags
    ADD COLUMN IF NOT EXISTS public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN IF NOT EXISTS label VARCHAR(150) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS organization_id BIGINT NULL REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS redirect_token VARCHAR(255) NOT NULL DEFAULT gen_random_uuid()::text,
    ADD COLUMN IF NOT EXISTS tap_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_tapped_at TIMESTAMPTZ NULL;

UPDATE nfc_tags n
SET user_id = COALESCE(n.user_id, c.user_id),
    organization_id = COALESCE(n.organization_id, c.organization_id),
    assigned_at = COALESCE(n.assigned_at, n.created_at)
FROM digital_cards c
WHERE n.card_id = c.id;

UPDATE nfc_tags SET status = 'disabled' WHERE status = 'inactive';
UPDATE nfc_tags SET status = 'unassigned' WHERE card_id IS NULL AND status = 'active';

ALTER TABLE nfc_tags DROP CONSTRAINT IF EXISTS nfc_tags_status_check;
ALTER TABLE nfc_tags ADD CONSTRAINT nfc_tags_status_check
    CHECK (status IN ('unassigned', 'active', 'disabled', 'lost', 'archived'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_nfc_tags_public_id ON nfc_tags(public_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nfc_tags_redirect_token ON nfc_tags(redirect_token);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_uid ON nfc_tags(uid);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_card_id ON nfc_tags(card_id);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_user_id ON nfc_tags(user_id);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_organization_id ON nfc_tags(organization_id);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_status ON nfc_tags(status);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_created_at ON nfc_tags(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_nfc_tags_last_tapped_at ON nfc_tags(last_tapped_at DESC);
