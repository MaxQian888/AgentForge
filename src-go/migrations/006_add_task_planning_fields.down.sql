DROP INDEX IF EXISTS idx_tasks_planned_start_at;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS planned_end_at,
    DROP COLUMN IF EXISTS planned_start_at;
