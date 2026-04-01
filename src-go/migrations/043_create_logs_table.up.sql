CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL,
    tab VARCHAR(20) NOT NULL DEFAULT 'system',
    level VARCHAR(10) NOT NULL DEFAULT 'info',
    actor_type VARCHAR(20) NOT NULL DEFAULT '',
    actor_id VARCHAR(255) NOT NULL DEFAULT '',
    agent_id UUID,
    session_id VARCHAR(255),
    event_type VARCHAR(100) NOT NULL DEFAULT '',
    action VARCHAR(100) NOT NULL DEFAULT '',
    resource_type VARCHAR(100) NOT NULL DEFAULT '',
    resource_id VARCHAR(255) NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    detail JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_logs_project_tab_created ON logs (project_id, tab, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_logs_project_level ON logs (project_id, level);
CREATE INDEX IF NOT EXISTS idx_logs_project_agent ON logs (project_id, agent_id) WHERE agent_id IS NOT NULL;
