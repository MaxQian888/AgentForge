ALTER TABLE reviews DROP COLUMN IF EXISTS execution_id;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS triggered_by;
ALTER TABLE agent_memory DROP COLUMN IF EXISTS employee_id;
ALTER TABLE agent_memory DROP CONSTRAINT IF EXISTS agent_memory_scope_check;
ALTER TABLE agent_memory
    ADD CONSTRAINT agent_memory_scope_check
    CHECK (scope IN ('global','project','role'));
ALTER TABLE agent_runs DROP COLUMN IF EXISTS employee_id;

DROP TABLE IF EXISTS workflow_triggers;
DROP TABLE IF EXISTS employee_skills;
DROP TABLE IF EXISTS employees;
