DROP TRIGGER IF EXISTS agent_runs_updated_at_trigger ON agent_runs;

ALTER TABLE agent_runs
    DROP COLUMN IF EXISTS updated_at;
