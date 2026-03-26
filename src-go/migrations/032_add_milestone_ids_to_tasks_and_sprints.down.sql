DROP INDEX IF EXISTS idx_sprints_milestone;
DROP INDEX IF EXISTS idx_tasks_milestone;

ALTER TABLE sprints
    DROP COLUMN IF EXISTS milestone_id;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS milestone_id;
