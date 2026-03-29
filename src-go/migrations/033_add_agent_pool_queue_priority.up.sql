ALTER TABLE agent_pool_queue_entries
    ADD COLUMN IF NOT EXISTS priority INT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_agent_pool_queue_entries_project_status_created;

CREATE INDEX IF NOT EXISTS idx_agent_pool_queue_entries_project_status_priority_created
    ON agent_pool_queue_entries(project_id, status, priority DESC, created_at ASC);
