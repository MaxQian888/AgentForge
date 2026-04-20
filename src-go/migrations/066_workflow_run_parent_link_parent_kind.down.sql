DROP INDEX IF EXISTS workflow_run_parent_link_parent_kind_idx;

ALTER TABLE workflow_run_parent_link
    DROP COLUMN IF EXISTS parent_kind;

-- Note: the FK constraint we dropped in the up migration is intentionally not
-- re-added. Restoring it would require all rows to still reference
-- workflow_executions; if plugin_run rows exist the restore would fail.
