DROP INDEX IF EXISTS idx_agent_pool_queue_entries_project_status_priority_created;

CREATE INDEX IF NOT EXISTS idx_agent_pool_queue_entries_project_status_created
    ON agent_pool_queue_entries(project_id, status, created_at);

ALTER TABLE agent_pool_queue_entries
    DROP COLUMN IF EXISTS priority;
