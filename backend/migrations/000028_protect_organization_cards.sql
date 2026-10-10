-- Preserve existing rows, including any legacy orphan IDs. Enforce the
-- relationship for new assignments and prevent deleting a referenced org.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'digital_cards'::regclass
          AND conname = 'digital_cards_organization_id_fkey'
    ) THEN
        ALTER TABLE digital_cards
            ADD CONSTRAINT digital_cards_organization_id_fkey
            FOREIGN KEY (organization_id) REFERENCES organizations(id)
            ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;
