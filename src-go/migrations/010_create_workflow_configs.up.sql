CREATE TABLE IF NOT EXISTS workflow_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    transitions JSONB NOT NULL DEFAULT '{}',
    triggers    JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id)
);

CREATE TRIGGER set_workflow_configs_updated_at
    BEFORE UPDATE ON workflow_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
