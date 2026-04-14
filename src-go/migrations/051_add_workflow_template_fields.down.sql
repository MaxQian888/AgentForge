DROP INDEX IF EXISTS idx_workflow_definitions_template;
DROP INDEX IF EXISTS idx_workflow_definitions_category;

ALTER TABLE workflow_definitions DROP CONSTRAINT IF EXISTS workflow_definitions_status_check;
ALTER TABLE workflow_definitions ADD CONSTRAINT workflow_definitions_status_check
  CHECK (status IN ('draft','active','archived'));

ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS source_id;
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS version;
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS template_vars;
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS category;
