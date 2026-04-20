DROP INDEX IF EXISTS idx_workflow_triggers_acting_employee;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS description;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS display_name;
ALTER TABLE workflow_triggers DROP COLUMN IF EXISTS created_via;
