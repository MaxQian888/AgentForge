-- 057_add_member_project_role.up.sql
-- Introduce members.project_role with strict CHECK constraint covering the
-- four canonical project roles. Backfills all existing rows to 'editor'.
-- Owner backfill is intentionally NOT automatic: the post-migration report
-- (migration_reports/2026-04-17-projects-without-owner.md) lists projects
-- that need manual operator intervention to assign an owner.
ALTER TABLE members
    ADD COLUMN IF NOT EXISTS project_role VARCHAR(16);

UPDATE members
SET project_role = 'editor'
WHERE project_role IS NULL OR project_role = '';

ALTER TABLE members
    ALTER COLUMN project_role SET DEFAULT 'editor';

ALTER TABLE members
    ALTER COLUMN project_role SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'members_project_role_check'
    ) THEN
        ALTER TABLE members
            ADD CONSTRAINT members_project_role_check
            CHECK (project_role IN ('owner', 'admin', 'editor', 'viewer'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_members_project_role
    ON members(project_id, project_role);
