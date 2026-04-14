ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS runtime TEXT NOT NULL DEFAULT '';

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT '';

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS model TEXT NOT NULL DEFAULT '';

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS role_id TEXT NOT NULL DEFAULT '';

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS queue_entry_id TEXT NOT NULL DEFAULT '';

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS queue_priority INT NULL;

ALTER TABLE dispatch_attempts
    ADD COLUMN IF NOT EXISTS recovery_disposition TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_pool_queue_entries
    ADD COLUMN IF NOT EXISTS guardrail_type TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_pool_queue_entries
    ADD COLUMN IF NOT EXISTS guardrail_scope TEXT NOT NULL DEFAULT '';

ALTER TABLE agent_pool_queue_entries
    ADD COLUMN IF NOT EXISTS recovery_disposition TEXT NOT NULL DEFAULT '';
