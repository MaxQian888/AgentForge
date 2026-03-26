CREATE TABLE automation_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    event_type VARCHAR(50) NOT NULL,
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failed', 'skipped')),
    detail JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_automation_logs_rule_triggered_at ON automation_logs(rule_id, triggered_at DESC);
CREATE INDEX idx_automation_logs_task ON automation_logs(task_id);
CREATE INDEX idx_automation_logs_status ON automation_logs(status);
CREATE INDEX idx_automation_logs_detail_gin ON automation_logs USING GIN(detail);
