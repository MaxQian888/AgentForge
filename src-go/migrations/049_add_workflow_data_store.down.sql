ALTER TABLE workflow_node_executions DROP CONSTRAINT IF EXISTS workflow_node_executions_status_check;
ALTER TABLE workflow_node_executions ADD CONSTRAINT workflow_node_executions_status_check
  CHECK (status IN ('pending','running','completed','failed','skipped'));

ALTER TABLE workflow_executions DROP CONSTRAINT IF EXISTS workflow_executions_status_check;
ALTER TABLE workflow_executions ADD CONSTRAINT workflow_executions_status_check
  CHECK (status IN ('pending','running','completed','failed','cancelled'));

ALTER TABLE workflow_node_executions DROP COLUMN IF EXISTS iteration_index;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS data_store;
