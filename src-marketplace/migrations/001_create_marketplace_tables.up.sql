CREATE TABLE IF NOT EXISTS marketplace_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type           TEXT NOT NULL CHECK (type IN ('plugin', 'skill', 'role')),
    slug           TEXT NOT NULL,
    name           TEXT NOT NULL,
    author_id      UUID NOT NULL,
    author_name    TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    category       TEXT NOT NULL DEFAULT '',
    tags           TEXT[] NOT NULL DEFAULT '{}',
    icon_url       TEXT,
    repository_url TEXT,
    license        TEXT NOT NULL DEFAULT 'MIT',
    extra_metadata JSONB NOT NULL DEFAULT '{}',
    latest_version TEXT,
    download_count BIGINT NOT NULL DEFAULT 0,
    avg_rating     NUMERIC(3,2) NOT NULL DEFAULT 0,
    rating_count   INTEGER NOT NULL DEFAULT 0,
    is_verified    BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured    BOOLEAN NOT NULL DEFAULT FALSE,
    is_deleted     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (type, slug)
);

CREATE TABLE IF NOT EXISTS marketplace_item_versions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id             UUID NOT NULL REFERENCES marketplace_items(id) ON DELETE CASCADE,
    version             TEXT NOT NULL,
    changelog           TEXT NOT NULL DEFAULT '',
    artifact_path       TEXT NOT NULL,
    artifact_size_bytes BIGINT NOT NULL DEFAULT 0,
    artifact_digest     TEXT NOT NULL,
    is_latest           BOOLEAN NOT NULL DEFAULT FALSE,
    is_yanked           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (item_id, version)
);

CREATE TABLE IF NOT EXISTS marketplace_reviews (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id    UUID NOT NULL REFERENCES marketplace_items(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL,
    user_name  TEXT NOT NULL,
    rating     SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (item_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_marketplace_items_type     ON marketplace_items(type);
CREATE INDEX IF NOT EXISTS idx_marketplace_items_category ON marketplace_items(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_items_featured ON marketplace_items(is_featured) WHERE is_featured = TRUE;
CREATE INDEX IF NOT EXISTS idx_marketplace_items_author   ON marketplace_items(author_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_item_versions  ON marketplace_item_versions(item_id);
CREATE INDEX IF NOT EXISTS idx_marketplace_reviews_item   ON marketplace_reviews(item_id);

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END;
$$ language 'plpgsql';

CREATE TRIGGER update_marketplace_items_updated_at
    BEFORE UPDATE ON marketplace_items FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_marketplace_reviews_updated_at
    BEFORE UPDATE ON marketplace_reviews FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
