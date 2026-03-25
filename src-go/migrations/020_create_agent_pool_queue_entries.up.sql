CREATE TABLE IF NOT EXISTS agent_pool_queue_entries (
    entry_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    runtime TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    role_id TEXT NULL,
    budget_usd DOUBLE PRECISION NOT NULL DEFAULT 0,
    agent_run_id UUID NULL REFERENCES agent_runs(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_pool_queue_entries_project_status_created
    ON agent_pool_queue_entries(project_id, status, created_at);

CREATE INDEX IF NOT EXISTS idx_agent_pool_queue_entries_task_status
    ON agent_pool_queue_entries(task_id, status);
