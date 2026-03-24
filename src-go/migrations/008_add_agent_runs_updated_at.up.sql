ALTER TABLE agent_runs
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE agent_runs
SET updated_at = COALESCE(completed_at, started_at, created_at, NOW())
WHERE updated_at IS NULL OR updated_at = created_at;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_trigger
        WHERE tgname = 'agent_runs_updated_at_trigger'
    ) THEN
        CREATE TRIGGER agent_runs_updated_at_trigger
            BEFORE UPDATE ON agent_runs
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END
$$;
