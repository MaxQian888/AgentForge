ALTER TABLE agent_pool_queue_entries
    DROP COLUMN IF EXISTS recovery_disposition;

ALTER TABLE agent_pool_queue_entries
    DROP COLUMN IF EXISTS guardrail_scope;

ALTER TABLE agent_pool_queue_entries
    DROP COLUMN IF EXISTS guardrail_type;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS recovery_disposition;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS queue_priority;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS queue_entry_id;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS role_id;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS model;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS provider;

ALTER TABLE dispatch_attempts
    DROP COLUMN IF EXISTS runtime;
