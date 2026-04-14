ALTER TABLE workflow_definitions ADD COLUMN category VARCHAR(30) NOT NULL DEFAULT 'user';
ALTER TABLE workflow_definitions ADD COLUMN template_vars JSONB NOT NULL DEFAULT '{}';
ALTER TABLE workflow_definitions ADD COLUMN version INT NOT NULL DEFAULT 1;
ALTER TABLE workflow_definitions ADD COLUMN source_id UUID REFERENCES workflow_definitions(id) ON DELETE SET NULL;

ALTER TABLE workflow_definitions DROP CONSTRAINT IF EXISTS workflow_definitions_status_check;
ALTER TABLE workflow_definitions ADD CONSTRAINT workflow_definitions_status_check
  CHECK (status IN ('draft','active','archived','template'));

CREATE INDEX idx_workflow_definitions_category ON workflow_definitions(category);
CREATE INDEX idx_workflow_definitions_template ON workflow_definitions(status) WHERE status = 'template';
