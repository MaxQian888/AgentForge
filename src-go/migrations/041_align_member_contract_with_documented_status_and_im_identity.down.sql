DROP INDEX IF EXISTS idx_members_im;

ALTER TABLE members
    DROP CONSTRAINT IF EXISTS members_status_check;

ALTER TABLE members
    DROP COLUMN IF EXISTS im_user_id;

ALTER TABLE members
    DROP COLUMN IF EXISTS im_platform;

ALTER TABLE members
    DROP COLUMN IF EXISTS status;
