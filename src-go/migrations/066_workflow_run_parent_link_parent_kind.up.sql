-- Add parent_kind discriminator so the sub-workflow linkage table can carry
-- parent-side engine information symmetric to child_engine_kind. Introduced by
-- bridge-legacy-to-dag-invocation so a legacy workflow plugin run can invoke a
-- DAG child through the `workflow` step action; the resume hook routes the
-- terminal-state notification back to the plugin runtime when parent_kind is
-- 'plugin_run', or to the DAG service when parent_kind is 'dag_execution'.
--
-- Values:
--   'dag_execution' — parent is a DAG workflow execution (existing rows)
--   'plugin_run'    — parent is a legacy workflow plugin run (new)
--
-- parent_run_id is not added as a separate column: for plugin parents the
-- parent_execution_id column is reinterpreted as the plugin run id. We drop
-- the FK constraint so the column can reference either workflow_executions.id
-- OR a plugin run id (plugin runs are currently persisted in memory).
ALTER TABLE workflow_run_parent_link
    DROP CONSTRAINT IF EXISTS workflow_run_parent_link_parent_execution_id_fkey;

ALTER TABLE workflow_run_parent_link
    ADD COLUMN parent_kind TEXT NOT NULL DEFAULT 'dag_execution';

CREATE INDEX workflow_run_parent_link_parent_kind_idx
    ON workflow_run_parent_link(parent_kind);
