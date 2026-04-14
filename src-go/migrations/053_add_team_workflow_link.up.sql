ALTER TABLE agent_teams ADD COLUMN workflow_execution_id UUID REFERENCES workflow_executions(id) ON DELETE SET NULL;
CREATE INDEX idx_agent_teams_workflow_exec ON agent_teams(workflow_execution_id) WHERE workflow_execution_id IS NOT NULL;
