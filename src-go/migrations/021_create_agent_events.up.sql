CREATE TABLE IF NOT EXISTS agent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id UUID NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    task_id UUID NOT NULL,
    project_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_events_run_id ON agent_events(run_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_task_id ON agent_events(task_id);
CREATE INDEX IF NOT EXISTS idx_agent_events_project_time ON agent_events(project_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_events_event_type ON agent_events(event_type);
