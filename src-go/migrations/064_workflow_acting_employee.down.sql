DROP INDEX IF EXISTS workflow_executions_acting_employee_idx;
ALTER TABLE workflow_executions
    DROP COLUMN IF EXISTS acting_employee_id;

DROP INDEX IF EXISTS workflow_triggers_acting_employee_idx;
ALTER TABLE workflow_triggers
    DROP COLUMN IF EXISTS acting_employee_id;
