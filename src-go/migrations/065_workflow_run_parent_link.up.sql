-- Parent↔child linkage for DAG sub_workflow invocations. Every time a parent
-- DAG execution's sub_workflow node spins up a child run (either another DAG
-- execution or a legacy workflow plugin run), a row is inserted here so the
-- engine can walk the invocation tree in both directions: parent reads the
-- rows keyed by (parent_execution_id, parent_node_id) to resume; child reads
-- the row keyed by (child_engine_kind, child_run_id) to discover its parent.
--
-- child_run_id is not a foreign key because it references one of two different
-- tables depending on child_engine_kind:
--   - 'dag'    → workflow_executions.id
--   - 'plugin' → WorkflowPluginRun aggregate (currently in-memory, not a table)
-- Keeping the column FK-free avoids coupling and lets either engine evolve
-- independently.
CREATE TABLE workflow_run_parent_link (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_execution_id  UUID NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE,
    parent_node_id       VARCHAR(120) NOT NULL,
    child_engine_kind    VARCHAR(16) NOT NULL,
    child_run_id         UUID NOT NULL,
    status               VARCHAR(32) NOT NULL DEFAULT 'running',
    started_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    terminated_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX workflow_run_parent_link_parent_node_idx
    ON workflow_run_parent_link(parent_execution_id, parent_node_id);

CREATE INDEX workflow_run_parent_link_child_idx
    ON workflow_run_parent_link(child_engine_kind, child_run_id);
