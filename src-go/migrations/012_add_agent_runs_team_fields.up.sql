ALTER TABLE agent_runs ADD COLUMN team_id UUID REFERENCES agent_teams(id) ON DELETE SET NULL;
ALTER TABLE agent_runs ADD COLUMN team_role VARCHAR(20) DEFAULT '';
CREATE INDEX idx_agent_runs_team ON agent_runs(team_id) WHERE team_id IS NOT NULL;
