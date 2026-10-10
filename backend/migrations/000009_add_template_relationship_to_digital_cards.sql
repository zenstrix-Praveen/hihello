CREATE INDEX IF NOT EXISTS idx_digital_cards_template_id
ON digital_cards(template_id);

ALTER TABLE digital_cards
ADD CONSTRAINT digital_cards_template_id_fkey
FOREIGN KEY (template_id) REFERENCES website_templates(id) ON DELETE SET NULL
NOT VALID;
