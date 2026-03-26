CREATE TABLE dashboard_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    layout JSONB NOT NULL DEFAULT '[]',
    created_by UUID REFERENCES members(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX idx_dashboard_configs_project ON dashboard_configs(project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_dashboard_configs_layout_gin ON dashboard_configs USING GIN(layout);

CREATE TRIGGER dashboard_configs_updated_at_trigger
    BEFORE UPDATE ON dashboard_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
