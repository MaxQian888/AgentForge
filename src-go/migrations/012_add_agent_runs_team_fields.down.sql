DROP INDEX IF EXISTS idx_agent_runs_team;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS team_role;
ALTER TABLE agent_runs DROP COLUMN IF EXISTS team_id;
