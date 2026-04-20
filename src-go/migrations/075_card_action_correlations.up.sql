CREATE TABLE card_action_correlations (
  token        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  execution_id uuid        NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
  node_id      text        NOT NULL,
  action_id    text        NOT NULL,
  payload      jsonb,
  expires_at   timestamptz NOT NULL,
  consumed_at  timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_cac_active
  ON card_action_correlations (expires_at)
  WHERE consumed_at IS NULL;

CREATE INDEX idx_cac_execution
  ON card_action_correlations (execution_id);
