-- 056_unify_knowledge_assets.up.sql
-- Unify wiki_pages and project_documents into knowledge_assets (STI pattern).

BEGIN;

-- ============================================================
-- 1. Create enums
-- ============================================================
DO $$ BEGIN
  CREATE TYPE knowledge_asset_kind AS ENUM ('wiki_page', 'ingested_file', 'template');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE ingest_status_enum AS ENUM ('pending', 'processing', 'ready', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ============================================================
-- 2. Create knowledge_assets table
-- ============================================================
CREATE TABLE IF NOT EXISTS knowledge_assets (
  id                  UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id          UUID          NOT NULL,
  wiki_space_id       UUID,
  parent_id           UUID,
  kind                knowledge_asset_kind NOT NULL,
  title               TEXT          NOT NULL,
  path                TEXT,
  sort_order          INT           NOT NULL DEFAULT 0,
  content_json        JSONB,
  content_text        TEXT,
  file_ref            TEXT,
  file_size           BIGINT,
  mime_type           TEXT,
  ingest_status       ingest_status_enum,
  ingest_chunk_count  INT           NOT NULL DEFAULT 0,
  template_category   TEXT,
  is_system_template  BOOLEAN       NOT NULL DEFAULT FALSE,
  is_pinned           BOOLEAN       NOT NULL DEFAULT FALSE,
  owner_id            UUID,
  created_by          UUID,
  updated_by          UUID,
  created_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  deleted_at          TIMESTAMPTZ,
  version             BIGINT        NOT NULL DEFAULT 1,
  search_vector       TSVECTOR
);

CREATE INDEX IF NOT EXISTS idx_knowledge_assets_project
  ON knowledge_assets(project_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_assets_space
  ON knowledge_assets(wiki_space_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_assets_parent
  ON knowledge_assets(parent_id)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_assets_kind
  ON knowledge_assets(kind)
  WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledge_assets_search
  ON knowledge_assets USING GIN(search_vector);

-- Auto-update tsvector on insert/update
CREATE OR REPLACE FUNCTION knowledge_assets_tsvector_update()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('english', COALESCE(NEW.title, '')), 'A') ||
    setweight(to_tsvector('english', COALESCE(NEW.content_text, '')), 'B');
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_knowledge_assets_tsvector ON knowledge_assets;
CREATE TRIGGER trg_knowledge_assets_tsvector
  BEFORE INSERT OR UPDATE OF title, content_text
  ON knowledge_assets
  FOR EACH ROW EXECUTE PROCEDURE knowledge_assets_tsvector_update();

-- ============================================================
-- 3. Create asset_versions table
-- ============================================================
CREATE TABLE IF NOT EXISTS asset_versions (
  id             UUID  PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id       UUID  NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
  version_number INT   NOT NULL,
  name           TEXT  NOT NULL,
  kind_snapshot  TEXT  NOT NULL,
  content_json   JSONB,
  file_ref       TEXT,
  created_by     UUID,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (asset_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_asset_versions_asset
  ON asset_versions(asset_id);

-- ============================================================
-- 4. Create asset_comments table
-- ============================================================
CREATE TABLE IF NOT EXISTS asset_comments (
  id                UUID  PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id          UUID  NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
  anchor_block_id   TEXT,
  parent_comment_id UUID,
  body              TEXT  NOT NULL,
  mentions          JSONB NOT NULL DEFAULT '[]',
  resolved_at       TIMESTAMPTZ,
  created_by        UUID,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_asset_comments_asset
  ON asset_comments(asset_id)
  WHERE deleted_at IS NULL;

-- ============================================================
-- 5. Create asset_ingest_chunks table
-- ============================================================
CREATE TABLE IF NOT EXISTS asset_ingest_chunks (
  id          UUID  PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id    UUID  NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
  chunk_index INT   NOT NULL,
  content     TEXT  NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (asset_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_asset_ingest_chunks_asset
  ON asset_ingest_chunks(asset_id);

-- ============================================================
-- 6. Migrate wiki_pages → knowledge_assets (kind=wiki_page / template)
--    Only run if wiki_pages table still exists.
-- ============================================================
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'wiki_pages') THEN
    INSERT INTO knowledge_assets (
      id, project_id, wiki_space_id, parent_id, kind,
      title, path, sort_order,
      content_json, content_text,
      template_category, is_system_template, is_pinned,
      created_by, updated_by, created_at, updated_at, deleted_at, version
    )
    SELECT
      wp.id,
      ws.project_id,
      wp.space_id,
      wp.parent_id,
      CASE WHEN wp.is_template THEN 'template'::knowledge_asset_kind
           ELSE 'wiki_page'::knowledge_asset_kind END,
      wp.title,
      wp.path,
      wp.sort_order,
      wp.content,
      wp.content_text,
      wp.template_category,
      wp.is_system,
      wp.is_pinned,
      wp.created_by,
      wp.updated_by,
      wp.created_at,
      wp.updated_at,
      wp.deleted_at,
      1
    FROM wiki_pages wp
    JOIN wiki_spaces ws ON ws.id = wp.space_id
    ON CONFLICT (id) DO NOTHING;
  END IF;
END $$;

-- ============================================================
-- 7. Migrate page_versions → asset_versions
-- ============================================================
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_versions') THEN
    INSERT INTO asset_versions (
      id, asset_id, version_number, name, kind_snapshot, content_json, created_by, created_at
    )
    SELECT
      pv.id,
      pv.page_id,
      pv.version_number,
      pv.name,
      'wiki_page',
      pv.content,
      pv.created_by,
      pv.created_at
    FROM page_versions pv
    WHERE EXISTS (SELECT 1 FROM knowledge_assets ka WHERE ka.id = pv.page_id)
    ON CONFLICT (asset_id, version_number) DO NOTHING;
  END IF;
END $$;

-- ============================================================
-- 8. Migrate page_comments → asset_comments
-- ============================================================
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_comments') THEN
    INSERT INTO asset_comments (
      id, asset_id, anchor_block_id, parent_comment_id, body, mentions,
      resolved_at, created_by, created_at, updated_at, deleted_at
    )
    SELECT
      pc.id,
      pc.page_id,
      pc.anchor_block_id,
      pc.parent_comment_id,
      pc.body,
      COALESCE(pc.mentions::jsonb, '[]'),
      pc.resolved_at,
      pc.created_by,
      pc.created_at,
      pc.updated_at,
      pc.deleted_at
    FROM page_comments pc
    WHERE EXISTS (SELECT 1 FROM knowledge_assets ka WHERE ka.id = pc.page_id)
    ON CONFLICT (id) DO NOTHING;
  END IF;
END $$;

-- ============================================================
-- 9. Migrate project_documents → knowledge_assets (kind=ingested_file)
-- ============================================================
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'project_documents') THEN
    INSERT INTO knowledge_assets (
      id, project_id, kind, title, file_ref, file_size, mime_type,
      ingest_status, ingest_chunk_count,
      created_at, updated_at, version
    )
    SELECT
      gen_random_uuid(),
      pd.project_id::uuid,
      'ingested_file'::knowledge_asset_kind,
      pd.name,
      pd.storage_key,
      pd.file_size,
      pd.file_type,
      pd.status::text::ingest_status_enum,
      pd.chunk_count,
      pd.created_at,
      pd.updated_at,
      1
    FROM project_documents pd
    ON CONFLICT DO NOTHING;
  END IF;
END $$;

-- ============================================================
-- 10. Drop old tables (wiki_pages, page_versions, page_comments,
--     project_documents). Skip if already gone.
-- ============================================================
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_comments') THEN
    DROP TABLE page_comments;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_versions') THEN
    DROP TABLE page_versions;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_recent_access') THEN
    DROP TABLE page_recent_access;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'page_favorites') THEN
    DROP TABLE page_favorites;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'wiki_pages') THEN
    DROP TABLE wiki_pages;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'wiki_spaces') THEN
    DROP TABLE wiki_spaces;
  END IF;
END $$;

DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'project_documents') THEN
    DROP TABLE project_documents;
  END IF;
END $$;

COMMIT;
