CREATE TYPE entity_link_type AS ENUM ('requirement', 'design', 'test', 'retro', 'reference', 'mention');

CREATE TABLE entity_links (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL,
    source_id UUID NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    link_type entity_link_type NOT NULL,
    anchor_block_id TEXT,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE task_comments (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    parent_comment_id UUID REFERENCES task_comments(id) ON DELETE SET NULL,
    body TEXT NOT NULL DEFAULT '',
    mentions JSONB NOT NULL DEFAULT '[]'::jsonb,
    resolved_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_entity_links_source ON entity_links (source_type, source_id);
CREATE INDEX idx_entity_links_target ON entity_links (target_type, target_id);
CREATE INDEX idx_task_comments_task_created ON task_comments (task_id, created_at);
