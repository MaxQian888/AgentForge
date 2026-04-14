CREATE TABLE workflow_definitions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status      VARCHAR(20) NOT NULL DEFAULT 'draft'
      CHECK (status IN ('draft','active','archived')),
    nodes       JSONB NOT NULL DEFAULT '[]',
    edges       JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_definitions_project ON workflow_definitions(project_id);
CREATE INDEX idx_workflow_definitions_status ON workflow_definitions(project_id, status);

CREATE TRIGGER set_workflow_definitions_updated_at
    BEFORE UPDATE ON workflow_definitions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE workflow_executions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id   UUID NOT NULL REFERENCES workflow_definitions(id) ON DELETE CASCADE,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id       UUID REFERENCES tasks(id) ON DELETE SET NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
      CHECK (status IN ('pending','running','completed','failed','cancelled')),
    current_nodes JSONB NOT NULL DEFAULT '[]',
    context       JSONB NOT NULL DEFAULT '{}',
    error_message TEXT NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_executions_workflow ON workflow_executions(workflow_id);
CREATE INDEX idx_workflow_executions_project ON workflow_executions(project_id);
CREATE INDEX idx_workflow_executions_status ON workflow_executions(status) WHERE status IN ('pending','running');

CREATE TRIGGER set_workflow_executions_updated_at
    BEFORE UPDATE ON workflow_executions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE workflow_node_executions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id  UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    node_id       VARCHAR(120) NOT NULL DEFAULT '',
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
      CHECK (status IN ('pending','running','completed','failed','skipped')),
    result        JSONB DEFAULT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_node_executions_execution ON workflow_node_executions(execution_id, created_at);
