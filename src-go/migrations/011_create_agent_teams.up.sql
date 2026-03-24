CREATE TABLE agent_teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    name TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) DEFAULT 'pending'
      CHECK (status IN ('pending','planning','executing','reviewing','completed','failed','cancelled')),
    strategy VARCHAR(30) DEFAULT 'planner_coder_reviewer',
    planner_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
    reviewer_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
    total_budget_usd NUMERIC(10,2) DEFAULT 10.00,
    total_spent_usd  NUMERIC(10,4) DEFAULT 0,
    config JSONB DEFAULT '{}',
    error_message TEXT DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_teams_project ON agent_teams(project_id);
CREATE INDEX idx_agent_teams_task ON agent_teams(task_id);
CREATE INDEX idx_agent_teams_status ON agent_teams(status) WHERE status NOT IN ('completed','failed','cancelled');
