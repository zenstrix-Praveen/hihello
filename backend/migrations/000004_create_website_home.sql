CREATE TABLE IF NOT EXISTS website_home (
    id BIGSERIAL PRIMARY KEY,

    hero_title TEXT NOT NULL DEFAULT '',
    hero_subtitle TEXT NOT NULL DEFAULT '',

    primary_button_text VARCHAR(150) NOT NULL DEFAULT '',
    primary_button_url TEXT NOT NULL DEFAULT '',

    secondary_button_text VARCHAR(150) NOT NULL DEFAULT '',
    secondary_button_url TEXT NOT NULL DEFAULT '',

    hero_image_url TEXT NOT NULL DEFAULT '',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO website_home (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
