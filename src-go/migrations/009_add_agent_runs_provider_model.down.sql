ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS model,
    DROP COLUMN IF EXISTS provider;
