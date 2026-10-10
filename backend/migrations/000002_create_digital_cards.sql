CREATE TABLE IF NOT EXISTS digital_cards (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    organization_id BIGINT NULL,
    name VARCHAR(150) NOT NULL,
    slug VARCHAR(180) NOT NULL UNIQUE,
    job_title VARCHAR(150),
    company_name VARCHAR(200),
    phone VARCHAR(50),
    email VARCHAR(255),
    website TEXT,
    address TEXT,
    photo_url TEXT,
    logo_url TEXT,
    primary_color VARCHAR(30) NOT NULL DEFAULT '#1267E8',
    template_id BIGINT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT digital_cards_status_check CHECK (status IN ('draft','published','suspended','archived'))
);

CREATE INDEX IF NOT EXISTS idx_digital_cards_user_id ON digital_cards(user_id);
CREATE INDEX IF NOT EXISTS idx_digital_cards_slug ON digital_cards(slug);

CREATE TABLE IF NOT EXISTS digital_card_social_links (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES digital_cards(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS digital_card_custom_links (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES digital_cards(id) ON DELETE CASCADE,
    label VARCHAR(150) NOT NULL,
    url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS digital_card_videos (
    id BIGSERIAL PRIMARY KEY,
    card_id BIGINT NOT NULL REFERENCES digital_cards(id) ON DELETE CASCADE,
    title VARCHAR(200),
    url TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
