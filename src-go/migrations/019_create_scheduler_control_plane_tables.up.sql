CREATE TABLE IF NOT EXISTS scheduled_jobs (
    job_key TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'system',
    schedule TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    execution_mode TEXT NOT NULL,
    overlap_policy TEXT NOT NULL,
    last_run_status TEXT NOT NULL DEFAULT '',
    last_run_at TIMESTAMPTZ NULL,
    next_run_at TIMESTAMPTZ NULL,
    last_run_summary TEXT NULL,
    last_error TEXT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scheduled_job_runs (
    run_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_key TEXT NOT NULL REFERENCES scheduled_jobs(job_key) ON DELETE CASCADE,
    trigger_source TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ NULL,
    summary TEXT NULL,
    error_message TEXT NULL,
    metrics JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_enabled ON scheduled_jobs(enabled);
CREATE INDEX IF NOT EXISTS idx_scheduled_jobs_next_run_at ON scheduled_jobs(next_run_at);
CREATE INDEX IF NOT EXISTS idx_scheduled_job_runs_job_key_started_at ON scheduled_job_runs(job_key, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_scheduled_job_runs_job_key_status ON scheduled_job_runs(job_key, status);
