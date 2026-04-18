-- 057_add_member_project_role.down.sql
DROP INDEX IF EXISTS idx_members_project_role;

ALTER TABLE members
    DROP CONSTRAINT IF EXISTS members_project_role_check;

ALTER TABLE members
    DROP COLUMN IF EXISTS project_role;
