-- Agent runs table (for MVP, non-partitioned for simplicity)
CREATE TABLE agent_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    member_id UUID REFERENCES members(id) ON DELETE SET NULL,
    session_id TEXT DEFAULT '',
    status VARCHAR(20) DEFAULT 'starting' CHECK (status IN (
        'starting', 'running', 'paused', 'completed', 'failed', 'cancelled', 'budget_exceeded'
    )),
    prompt TEXT DEFAULT '',
    system_prompt TEXT DEFAULT '',
    worktree_path TEXT DEFAULT '',
    branch_name TEXT DEFAULT '',
    role_id TEXT DEFAULT '',
    input_tokens BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    cache_read_tokens BIGINT DEFAULT 0,
    cost_usd NUMERIC(10,4) DEFAULT 0,
    budget_usd NUMERIC(10,2) DEFAULT 5.00,
    turn_count INTEGER DEFAULT 0,
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_agent_runs_task ON agent_runs(task_id);
CREATE INDEX idx_agent_runs_member ON agent_runs(member_id);
CREATE INDEX idx_agent_runs_status ON agent_runs(status);
CREATE INDEX idx_agent_runs_active ON agent_runs(status) WHERE status IN ('starting', 'running', 'paused');

-- Reviews table
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID REFERENCES tasks(id) ON DELETE CASCADE,
    pr_url TEXT DEFAULT '',
    pr_number INTEGER,
    layer INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'in_progress', 'completed', 'failed')),
    risk_level VARCHAR(10) DEFAULT 'low' CHECK (risk_level IN ('critical', 'high', 'medium', 'low')),
    findings JSONB DEFAULT '[]',
    summary TEXT DEFAULT '',
    recommendation VARCHAR(20) DEFAULT '' CHECK (recommendation IN ('', 'approve', 'request_changes', 'reject')),
    cost_usd NUMERIC(10,4) DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reviews_task ON reviews(task_id);
CREATE INDEX idx_reviews_pr ON reviews(pr_url);

-- Notifications table
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_id UUID NOT NULL,
    target_type VARCHAR(10) DEFAULT 'user' CHECK (target_type IN ('user', 'member', 'project')),
    channel VARCHAR(20) DEFAULT 'web' CHECK (channel IN ('web', 'im', 'email')),
    type VARCHAR(50) DEFAULT 'info',
    title TEXT DEFAULT '',
    body TEXT DEFAULT '',
    data JSONB DEFAULT '{}',
    is_read BOOLEAN DEFAULT FALSE,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_notifications_target ON notifications(target_id, is_read);
CREATE INDEX idx_notifications_target_type ON notifications(target_id, target_type);
CREATE INDEX idx_notifications_created ON notifications(created_at DESC);
