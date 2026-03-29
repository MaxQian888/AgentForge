CREATE TABLE IF NOT EXISTS dispatch_attempts (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    member_id UUID NULL REFERENCES members(id) ON DELETE SET NULL,
    outcome TEXT NOT NULL,
    trigger_source TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    guardrail_type TEXT NOT NULL DEFAULT '',
    guardrail_scope TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dispatch_attempts_project_created
    ON dispatch_attempts(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_dispatch_attempts_task_created
    ON dispatch_attempts(task_id, created_at DESC);
