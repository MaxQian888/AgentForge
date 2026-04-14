-- Add DataStore to workflow executions for node-to-node data flow.
ALTER TABLE workflow_executions ADD COLUMN data_store JSONB NOT NULL DEFAULT '{}';

-- Add iteration tracking for loop nodes.
ALTER TABLE workflow_node_executions ADD COLUMN iteration_index INT NOT NULL DEFAULT 0;

-- Extend allowed statuses for paused workflows and waiting nodes.
ALTER TABLE workflow_executions DROP CONSTRAINT IF EXISTS workflow_executions_status_check;
ALTER TABLE workflow_executions ADD CONSTRAINT workflow_executions_status_check
  CHECK (status IN ('pending','running','completed','failed','cancelled','paused'));

ALTER TABLE workflow_node_executions DROP CONSTRAINT IF EXISTS workflow_node_executions_status_check;
ALTER TABLE workflow_node_executions ADD CONSTRAINT workflow_node_executions_status_check
  CHECK (status IN ('pending','running','completed','failed','skipped','waiting'));
