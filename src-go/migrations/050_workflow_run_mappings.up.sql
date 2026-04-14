CREATE TABLE workflow_run_mappings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id  UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    node_id       VARCHAR(120) NOT NULL,
    agent_run_id  UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_run_mappings_run ON workflow_run_mappings(agent_run_id);
CREATE INDEX idx_workflow_run_mappings_exec ON workflow_run_mappings(execution_id);
