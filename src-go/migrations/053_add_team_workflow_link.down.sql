DROP INDEX IF EXISTS idx_agent_teams_workflow_exec;
ALTER TABLE agent_teams DROP COLUMN IF EXISTS workflow_execution_id;
