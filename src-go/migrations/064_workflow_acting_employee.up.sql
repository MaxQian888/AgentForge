-- Add run-level acting-employee attribution to the workflow trigger row and the
-- DAG workflow execution record.  Both columns are nullable references to
-- employees(id); ON DELETE SET NULL so hard-deleting an employee does not
-- destroy historical run records.
--
-- Note: the legacy workflow plugin run aggregate (WorkflowPluginRun) is held in
-- an in-memory repository today, so no legacy-plugin-run table migration is
-- required.  The Go struct gains an ActingEmployeeID field in the same change.
ALTER TABLE workflow_triggers
    ADD COLUMN acting_employee_id UUID REFERENCES employees(id) ON DELETE SET NULL;

CREATE INDEX workflow_triggers_acting_employee_idx
    ON workflow_triggers(acting_employee_id)
    WHERE acting_employee_id IS NOT NULL;

ALTER TABLE workflow_executions
    ADD COLUMN acting_employee_id UUID REFERENCES employees(id) ON DELETE SET NULL;

CREATE INDEX workflow_executions_acting_employee_idx
    ON workflow_executions(acting_employee_id)
    WHERE acting_employee_id IS NOT NULL;
