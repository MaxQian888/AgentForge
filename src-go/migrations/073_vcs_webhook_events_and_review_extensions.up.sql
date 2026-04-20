CREATE TABLE IF NOT EXISTS vcs_webhook_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  integration_id uuid NOT NULL REFERENCES vcs_integrations(id) ON DELETE CASCADE,
  event_id varchar(128) NOT NULL,
  event_type varchar(32) NOT NULL,
  payload_hash bytea NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  processed_at timestamptz,
  processing_error text,
  UNIQUE (integration_id, event_id)
);
CREATE INDEX idx_vcs_webhook_events_received_at ON vcs_webhook_events(received_at DESC);

ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS integration_id uuid REFERENCES vcs_integrations(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS head_sha varchar(40),
  ADD COLUMN IF NOT EXISTS base_sha varchar(40),
  ADD COLUMN IF NOT EXISTS last_reviewed_sha varchar(40),
  ADD COLUMN IF NOT EXISTS summary_comment_id varchar(64),
  ADD COLUMN IF NOT EXISTS automation_decision varchar(16) NOT NULL DEFAULT 'manual_only';

ALTER TABLE review_findings
  ADD COLUMN IF NOT EXISTS suggested_patch text,
  ADD COLUMN IF NOT EXISTS decision varchar(16) NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS decided_at timestamptz,
  ADD COLUMN IF NOT EXISTS decided_by uuid,
  ADD COLUMN IF NOT EXISTS inline_comment_id varchar(64),
  ADD COLUMN IF NOT EXISTS active_fix_run_id uuid;
-- active_fix_run_id FK is added later by Plan 2D (which creates fix_runs).
