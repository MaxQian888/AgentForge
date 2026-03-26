CREATE TABLE form_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    fields JSONB NOT NULL DEFAULT '[]',
    target_status VARCHAR(30) DEFAULT 'inbox',
    target_assignee UUID REFERENCES members(id) ON DELETE SET NULL,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_form_definitions_active_slug ON form_definitions(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_form_definitions_project ON form_definitions(project_id);

CREATE TRIGGER form_definitions_updated_at_trigger
    BEFORE UPDATE ON form_definitions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
