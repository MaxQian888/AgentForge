CREATE TABLE workflow_pending_reviews (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id  UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    node_id       VARCHAR(120) NOT NULL,
    project_id    UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    reviewer_id   UUID REFERENCES members(id) ON DELETE SET NULL,
    prompt        TEXT NOT NULL DEFAULT '',
    context       JSONB NOT NULL DEFAULT '{}',
    decision      VARCHAR(20) NOT NULL DEFAULT 'pending'
      CHECK (decision IN ('pending','approved','rejected')),
    comment       TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);

CREATE INDEX idx_workflow_pending_reviews_exec ON workflow_pending_reviews(execution_id);
CREATE INDEX idx_workflow_pending_reviews_pending ON workflow_pending_reviews(project_id, decision) WHERE decision = 'pending';
