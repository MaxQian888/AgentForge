CREATE TABLE IF NOT EXISTS wiki_spaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE(project_id)
);

CREATE TABLE IF NOT EXISTS wiki_pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL REFERENCES wiki_spaces(id) ON DELETE CASCADE,
    parent_id UUID NULL REFERENCES wiki_pages(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    content JSONB NOT NULL DEFAULT '[]',
    content_text TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_template BOOLEAN NOT NULL DEFAULT FALSE,
    template_category TEXT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS page_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    version_number INT NOT NULL,
    name TEXT NOT NULL,
    content JSONB NOT NULL DEFAULT '[]',
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS page_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    page_id UUID NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    anchor_block_id TEXT NULL,
    parent_comment_id UUID NULL REFERENCES page_comments(id) ON DELETE SET NULL,
    body TEXT NOT NULL,
    mentions JSONB NOT NULL DEFAULT '[]',
    resolved_at TIMESTAMPTZ NULL,
    created_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS page_favorites (
    page_id UUID NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (page_id, user_id)
);

CREATE TABLE IF NOT EXISTS page_recent_access (
    page_id UUID NOT NULL REFERENCES wiki_pages(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (page_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_wiki_pages_content_gin ON wiki_pages USING GIN (content);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_path ON wiki_pages(path);
CREATE INDEX IF NOT EXISTS idx_wiki_pages_space_parent_sort ON wiki_pages(space_id, parent_id, sort_order);
CREATE UNIQUE INDEX IF NOT EXISTS idx_page_favorites_page_user ON page_favorites(page_id, user_id);
