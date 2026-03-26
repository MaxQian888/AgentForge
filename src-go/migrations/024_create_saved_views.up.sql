CREATE TABLE saved_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    owner_id UUID REFERENCES members(id) ON DELETE SET NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    shared_with JSONB NOT NULL DEFAULT '{}',
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_saved_views_project ON saved_views(project_id);
CREATE INDEX idx_saved_views_project_default ON saved_views(project_id, is_default) WHERE deleted_at IS NULL;
CREATE INDEX idx_saved_views_shared_with_gin ON saved_views USING GIN(shared_with);

CREATE TRIGGER saved_views_updated_at_trigger
    BEFORE UPDATE ON saved_views
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
