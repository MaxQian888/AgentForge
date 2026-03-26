CREATE TABLE custom_field_defs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    field_type VARCHAR(20) NOT NULL CHECK (field_type IN (
        'text', 'number', 'select', 'multi_select', 'date', 'user', 'url', 'checkbox'
    )),
    options JSONB NOT NULL DEFAULT '[]',
    sort_order INTEGER NOT NULL DEFAULT 0,
    required BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_custom_field_defs_project ON custom_field_defs(project_id);
CREATE INDEX idx_custom_field_defs_project_sort ON custom_field_defs(project_id, sort_order) WHERE deleted_at IS NULL;

CREATE TRIGGER custom_field_defs_updated_at_trigger
    BEFORE UPDATE ON custom_field_defs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
