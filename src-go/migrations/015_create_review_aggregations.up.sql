CREATE TABLE review_aggregations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pr_url TEXT NOT NULL,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    review_ids UUID[] NOT NULL DEFAULT '{}',
    overall_risk VARCHAR(16) NOT NULL DEFAULT 'low',
    recommendation VARCHAR(32) NOT NULL DEFAULT '',
    findings JSONB NOT NULL DEFAULT '[]',
    summary TEXT NOT NULL DEFAULT '',
    metrics JSONB NOT NULL DEFAULT '{}',
    human_decision VARCHAR(32),
    human_reviewer UUID,
    human_comment TEXT,
    decided_at TIMESTAMPTZ,
    total_cost_usd NUMERIC(12,6) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_review_aggregations_task ON review_aggregations(task_id);
CREATE INDEX idx_review_aggregations_pr ON review_aggregations(pr_url);
