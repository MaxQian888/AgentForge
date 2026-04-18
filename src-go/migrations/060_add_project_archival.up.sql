-- 060_add_project_archival.up.sql
-- Promote `status` to a stored column + add archival bookkeeping.
--
-- Previously `projects.status` was a virtual field defaulted to 'active' in
-- application code. Archival requires durable state so RBAC can gate writes
-- against archived projects and the list API can hide them by default.
--
-- New columns:
--   status                 : VARCHAR(16) NOT NULL, enum {active,paused,archived}
--   archived_at            : TIMESTAMPTZ NULL — set when Archive transitions
--   archived_by_user_id    : UUID NULL       — owner who archived the project
--
-- See:
--   - openspec/changes/2026-04-17-add-project-archival/design.md
--   - openspec/specs/project-management-api-contracts (status enum)

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS status VARCHAR(16);

UPDATE projects
SET status = 'active'
WHERE status IS NULL OR status = '';

ALTER TABLE projects
    ALTER COLUMN status SET DEFAULT 'active';

ALTER TABLE projects
    ALTER COLUMN status SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'projects_status_check'
    ) THEN
        ALTER TABLE projects
            ADD CONSTRAINT projects_status_check
            CHECK (status IN ('active', 'paused', 'archived'));
    END IF;
END $$;

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS archived_by_user_id UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_projects_status_archived_at
    ON projects(status, archived_at);
