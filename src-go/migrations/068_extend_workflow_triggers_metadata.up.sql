-- Extend workflow_triggers with the source-of-truth + display metadata
-- columns that Spec 1C needs for its FE CRUD surface (display_name /
-- description) and that the registrar needs to merge instead of
-- delete-then-insert (created_via discriminates DAG-owned vs human-authored
-- rows; only 'dag_node' rows are eligible to be replaced on workflow save).
ALTER TABLE workflow_triggers
    ADD COLUMN created_via VARCHAR(16) NOT NULL DEFAULT 'dag_node'
        CHECK (created_via IN ('dag_node','manual')),
    ADD COLUMN display_name VARCHAR(128),
    ADD COLUMN description TEXT;

-- Existing rows (all registrar-authored before Spec 1C lands) inherit the
-- 'dag_node' default, which is the desired backfill: their FE CRUD surface
-- is read-only until the user explicitly converts them to 'manual'.

-- Migration 064 already created a partial index on acting_employee_id with
-- a WHERE clause. The /api/v1/employees/:id/runs endpoint and the
-- registrar's per-employee filter both use the column without the IS NOT
-- NULL predicate, so we add an unconditional index here for plan-stability.
CREATE INDEX IF NOT EXISTS idx_workflow_triggers_acting_employee
    ON workflow_triggers(acting_employee_id);
