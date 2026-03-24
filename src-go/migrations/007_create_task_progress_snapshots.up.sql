CREATE TABLE IF NOT EXISTS task_progress_snapshots (
    task_id UUID PRIMARY KEY REFERENCES tasks(id) ON DELETE CASCADE,
    last_activity_at TIMESTAMPTZ NOT NULL,
    last_activity_source TEXT NOT NULL DEFAULT 'task_created',
    last_transition_at TIMESTAMPTZ NOT NULL,
    health_status TEXT NOT NULL DEFAULT 'healthy',
    risk_reason TEXT NOT NULL DEFAULT '',
    risk_since_at TIMESTAMPTZ,
    last_alert_state TEXT NOT NULL DEFAULT '',
    last_alert_at TIMESTAMPTZ,
    last_recovered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO task_progress_snapshots (
    task_id,
    last_activity_at,
    last_activity_source,
    last_transition_at,
    health_status,
    risk_reason,
    risk_since_at,
    last_alert_state,
    last_alert_at,
    last_recovered_at,
    created_at,
    updated_at
)
SELECT
    tasks.id,
    COALESCE(tasks.updated_at, tasks.created_at),
    CASE
        WHEN tasks.updated_at > tasks.created_at THEN 'task_updated'
        ELSE 'task_created'
    END,
    COALESCE(tasks.completed_at, tasks.updated_at, tasks.created_at),
    'healthy',
    '',
    NULL,
    '',
    NULL,
    NULL,
    NOW(),
    NOW()
FROM tasks
WHERE tasks.status NOT IN ('done', 'cancelled')
ON CONFLICT (task_id) DO NOTHING;
