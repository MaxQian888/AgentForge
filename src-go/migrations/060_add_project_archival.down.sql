-- 060_add_project_archival.down.sql
DROP INDEX IF EXISTS idx_projects_status_archived_at;

ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_status_check;

ALTER TABLE projects
    DROP COLUMN IF EXISTS archived_by_user_id;

ALTER TABLE projects
    DROP COLUMN IF EXISTS archived_at;

ALTER TABLE projects
    DROP COLUMN IF EXISTS status;
