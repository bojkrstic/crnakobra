-- Budući javni sadržaj: vesti, kampovi i slike za galeriju.
-- Samo objavljeni zapisi će biti vidljivi na javnim rutama.
CREATE TABLE content_items (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('news', 'camp', 'gallery')),
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    summary TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    event_date DATE,
    location TEXT NOT NULL DEFAULT '',
    published BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX content_items_public_index
    ON content_items (kind, published, published_at DESC, created_at DESC);
