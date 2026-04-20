ALTER TABLE review_findings
  DROP COLUMN IF EXISTS active_fix_run_id,
  DROP COLUMN IF EXISTS inline_comment_id,
  DROP COLUMN IF EXISTS decided_by,
  DROP COLUMN IF EXISTS decided_at,
  DROP COLUMN IF EXISTS decision,
  DROP COLUMN IF EXISTS suggested_patch;

ALTER TABLE reviews
  DROP COLUMN IF EXISTS automation_decision,
  DROP COLUMN IF EXISTS summary_comment_id,
  DROP COLUMN IF EXISTS last_reviewed_sha,
  DROP COLUMN IF EXISTS base_sha,
  DROP COLUMN IF EXISTS head_sha,
  DROP COLUMN IF EXISTS integration_id;

DROP INDEX IF EXISTS idx_vcs_webhook_events_received_at;
DROP TABLE IF EXISTS vcs_webhook_events;
