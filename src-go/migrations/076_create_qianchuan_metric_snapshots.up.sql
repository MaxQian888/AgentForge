-- Per-binding minute-bucketed metric snapshots.
-- The (binding_id, minute_bucket) UNIQUE supports UPSERT on the polling
-- path so duplicate ticks within the same minute are idempotent.
CREATE TABLE IF NOT EXISTS qianchuan_metric_snapshots (
    id            BIGSERIAL PRIMARY KEY,
    binding_id    UUID NOT NULL REFERENCES qianchuan_bindings(id) ON DELETE CASCADE,
    minute_bucket TIMESTAMPTZ NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, minute_bucket)
);
CREATE INDEX IF NOT EXISTS idx_qms_binding_time
    ON qianchuan_metric_snapshots (binding_id, minute_bucket DESC);
