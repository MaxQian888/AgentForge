-- Per-action audit row. status enum:
--   'pending'  — emitted by strategy_runner, not yet acted on
--   'gated'    — policy_gate (3E) blocked or routed for human approval
--   'applied'  — action_executor applied successfully
--   'rejected' — operator rejected via approval card
--   'failed'   — provider call returned an error
CREATE TABLE IF NOT EXISTS qianchuan_action_logs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    binding_id       UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
    strategy_id      UUID REFERENCES qianchuan_strategies(id) ON DELETE SET NULL,
    strategy_run_id  UUID NOT NULL,
    rule_name        VARCHAR(128),
    action_type      VARCHAR(32) NOT NULL,
    target_ad_id     VARCHAR(64),
    params           JSONB NOT NULL,
    status           VARCHAR(16) NOT NULL DEFAULT 'pending',
    gate_reason      VARCHAR(128),
    applied_at       TIMESTAMPTZ,
    error_message    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_qal_binding_time
    ON qianchuan_action_logs (binding_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_qal_run
    ON qianchuan_action_logs (strategy_run_id);
